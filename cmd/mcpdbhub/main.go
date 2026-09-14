package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/server"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/version"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "verify-metadata" {
		if err := verifyBackup(os.Args[2:], os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Printf("ContextGate %s (commit %s)\n", version.Version, version.BuildCommit())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		if e := resetPassword(os.Args[2:], os.Stdin, os.Stdout); e != nil {
			log.Fatal(e)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "stdio" {
		if e := stdio(os.Args[2:]); e != nil {
			log.Fatal(e)
		}
		return
	}
	if e := serve(); e != nil {
		log.Fatal(e)
	}
}
func serve() error {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	f := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := f.String("listen", env("MCPDBHUB_LISTEN", "127.0.0.1:8080"), "HTTP listen address")
	data := f.String("data-dir", env("MCPDBHUB_DATA_DIR", "./data"), "local encryption key directory")
	databaseURL := f.String("database-url", "", "PostgreSQL metadata connection (prefer MCPDBHUB_DATABASE_URL)")
	public := f.String("public-url", env("MCPDBHUB_PUBLIC_URL", "http://127.0.0.1:8080"), "canonical public HTTP(S) origin")
	files := f.String("database-dir", os.Getenv("MCPDBHUB_DATABASE_DIR"), "allowed directory for SQLite/DuckDB files")
	f.Parse(args)
	if *databaseURL == "" {
		*databaseURL = os.Getenv("MCPDBHUB_DATABASE_URL")
	}
	dir, e := filepath.Abs(*data)
	if e != nil {
		return e
	}
	if *files == "" {
		*files = filepath.Join(dir, "databases")
	}
	if e = os.MkdirAll(*files, 0700); e != nil {
		return e
	}
	st, e := store.Open(dir, *databaseURL)
	if e != nil {
		return e
	}
	defer st.Close()
	app, e := server.New(st, *public, *files)
	if e != nil {
		return e
	}
	defer app.Close()
	setup, e := app.EnsureSetup()
	if e != nil {
		return e
	}
	if setup != "" {
		fmt.Fprintf(os.Stderr, "First-time setup token: %s\nOpen %s to set the administrator password.\n", setup, *public)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go st.Cleanup(ctx)
	srv := &http.Server{Addr: *listen, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	done := make(chan error, 1)
	go func() { log.Printf("ContextGate listening on %s", *listen); done <- srv.ListenAndServe() }()
	select {
	case e := <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
