package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
)

func resetPassword(args []string, input io.Reader, output io.Writer) error {
	f := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	f.SetOutput(output)
	dir := f.String("data-dir", env("MCPDBHUB_DATA_DIR", "./data"), "existing configuration directory")
	databaseURL := f.String("database-url", "", "PostgreSQL metadata connection (prefer MCPDBHUB_DATABASE_URL)")
	stdin := f.Bool("password-stdin", false, "read the new password from standard input, without logging it")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *databaseURL == "" {
		*databaseURL = os.Getenv("MCPDBHUB_DATABASE_URL")
	}
	if !*stdin || f.NArg() != 0 {
		return errors.New("use reset-password --data-dir DIR --password-stdin; supply the new password through stdin, never as a command argument")
	}
	if os.Getenv("MCPDBHUB_MASTER_KEY") == "" {
		if info, err := os.Stat(filepath.Join(*dir, "master.key")); err != nil || !info.Mode().IsRegular() {
			return errors.New("master key does not exist; verify --data-dir or MCPDBHUB_MASTER_KEY")
		}
	}
	b, err := io.ReadAll(io.LimitReader(input, 259))
	if err != nil {
		return err
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
	if len(password) < 12 || len(password) > 256 {
		return errors.New("password must contain 12–256 bytes")
	}
	st, err := store.Open(*dir, *databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if _, err = st.Sources(); err != nil {
		return errors.New("cannot decrypt existing configuration; verify the master key")
	}
	if err = st.ReplaceAdminPassword(secure.Password(password)); err != nil {
		return err
	}
	fmt.Fprintln(output, "Administrator password reset. All administrator sessions were signed out. Data sources, Agent credentials and audit history are retained.")
	return nil
}
