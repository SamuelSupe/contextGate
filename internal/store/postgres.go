package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schema string

// Open initializes PostgreSQL metadata storage and verifies its independent
// encryption key. It never imports or opens an old SQLite configuration file.
func Open(dir, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("MCPDBHUB_DATABASE_URL is required for PostgreSQL metadata storage")
	}
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL metadata connection configuration")
	}
	cfg.ConnectTimeout = 10 * time.Second
	cfg.RuntimeParams["application_name"] = "contextgate"
	cfg.RuntimeParams["statement_timeout"] = "30000"
	cfg.RuntimeParams["lock_timeout"] = "10000"
	cfg.RuntimeParams["idle_in_transaction_session_timeout"] = "30000"
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	st := &Store{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err == nil {
		err = st.initialize(ctx, dir)
	}
	if err != nil {
		db.Close()
		// Connection errors may include addresses or credentials from the DSN.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, fmt.Errorf("PostgreSQL metadata initialization failed (SQLSTATE %s)", pgErr.Code)
		}
		var connectErr *pgconn.ConnectError
		if errors.As(err, &connectErr) {
			return nil, errors.New("cannot connect to PostgreSQL metadata storage; check MCPDBHUB_DATABASE_URL, TLS and database availability")
		}
		return nil, err
	}
	return st, nil
}

func (s *Store) initialize(ctx context.Context, dir string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize first-time DDL and key registration across simultaneous starts.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1296257090)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	var check string
	err = tx.QueryRowContext(ctx, "SELECT value FROM kv WHERE key='master_key_check'").Scan(&check)
	existing := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	vault, err := secure.OpenVault(dir, existing)
	if err != nil {
		return err
	}
	if existing {
		plain, err := vault.Open(check, "master-key-check")
		if err != nil || string(plain) != "mcpdbhub" {
			return errors.New("master key does not match PostgreSQL metadata; restore the original master.key or MCPDBHUB_MASTER_KEY")
		}
	} else if _, err = tx.ExecContext(ctx, "INSERT INTO kv(key,value) VALUES('master_key_check',$1)", vault.Seal([]byte("mcpdbhub"), "master-key-check")); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.Vault = vault
	return s.retainCurrentSemantics()
}
