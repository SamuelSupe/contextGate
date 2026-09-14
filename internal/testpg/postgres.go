package testpg

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DSN isolates each temporary configuration in its own PostgreSQL schema while
// letting persistence tests reopen the same schema using the same directory.
// The test database must be disposable and allow CREATE/DROP SCHEMA.
func DSN(t testing.TB, dir string) string {
	t.Helper()
	raw := os.Getenv("MCPDBHUB_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("set MCPDBHUB_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" || u.Host == "" {
		t.Fatal("MCPDBHUB_TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	db, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal("open test PostgreSQL connection")
	}
	digest := sha256.Sum256([]byte(dir))
	name := fmt.Sprintf("hubtest_%x", digest[:16])
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+name); err != nil {
		db.Close()
		t.Fatal("create isolated PostgreSQL test schema:", err)
	}
	t.Cleanup(func() {
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+name+" CASCADE"); err != nil {
			t.Error("clean PostgreSQL test schema:", err)
		}
	})
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	return u.String()
}
