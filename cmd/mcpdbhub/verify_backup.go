package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"time"

	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/version"
)

func verifyBackup(args []string, out io.Writer) error {
	f := flag.NewFlagSet("verify-metadata", flag.ContinueOnError)
	data := f.String("data-dir", env("MCPDBHUB_DATA_DIR", "./data"), "directory containing the backup encryption key")
	if err := f.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	counts, err := store.VerifyBackup(ctx, *data, os.Getenv("MCPDBHUB_DATABASE_URL"))
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(map[string]any{"status": "passed", "checked_at": time.Now().UTC(), "version": version.Version, "commit": version.BuildCommit(), "records": counts, "source_connections_opened": 0})
}
