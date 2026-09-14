package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/testpg"
)

func TestPostgresMetadataRequiresMatchingMasterKey(t *testing.T) {
	dir := t.TempDir()
	dsn := testpg.DSN(t, dir)
	s, err := Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSource(model.Source{ID: "kept", Password: "private-database-credential"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	path := filepath.Join(dir, "master.key")
	key, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir, dsn); err == nil {
		s.Close()
		t.Fatal("existing PostgreSQL metadata accepted a missing master key")
	}
	if err := os.WriteFile(path, make([]byte, 32), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir, dsn); err == nil {
		s.Close()
		t.Fatal("existing PostgreSQL metadata accepted an unrelated master key")
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src, err := s.Source("kept")
	if err != nil || src.Password != "private-database-credential" {
		t.Fatal("metadata did not recover with its original key", err)
	}
	if _, err := Open(dir, "postgres://user:private-dsn-password@host:invalid/db"); err == nil || strings.Contains(err.Error(), "private-dsn-password") {
		t.Fatal("invalid connection accepted or credential leaked")
	}
}

func TestPostgresSessionCannotCrossPasswordRevocation(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, testpg.DSN(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Setup("old-hash"); err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE kv SET value='new-hash' WHERE key='admin_password'"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("DELETE FROM sessions"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Session("late-login", "csrf", time.Now().Add(time.Hour), "old-hash") }()
	waitForMetadataLock(t, s, done, "SELECT value FROM kv WHERE key='admin_password' FOR SHARE")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; model.ErrorCode(err) != "unauthorized" {
		t.Fatal("stale login was not rejected after password commit", err)
	}
	if _, err := s.CheckSession("late-login"); err == nil {
		t.Fatal("stale login recreated a revoked session")
	}
}

func TestPostgresAuditExportDoesNotSkipConcurrentCommit(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, testpg.DSN(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO audit(at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code) VALUES(1,'first','source','query_sql','',0,0,'')`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- s.Audit(model.Audit{At: time.Now(), AgentID: "second", SourceID: "source", Operation: "query_sql"})
	}()
	waitForMetadataLock(t, s, done, "LOCK TABLE audit IN SHARE ROW EXCLUSIVE MODE")
	batch, err := s.AuditBatch(context.Background(), 0)
	if err != nil || len(batch) != 0 {
		t.Fatal("a later audit committed before the earlier record", batch, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	batch, err = s.AuditBatch(context.Background(), 0)
	if err != nil || len(batch) != 2 || batch[0].AgentID != "first" || batch[1].AgentID != "second" {
		t.Fatal("audit cursor order lost a committed event", batch, err)
	}
	page, err := s.Audits(AuditFilter{Agent: "second", Source: "source", Status: "success", From: time.Now().Add(-time.Hour)}, 1)
	if err != nil || len(page) != 1 || page[0].ID != batch[1].ID {
		t.Fatal("combined audit filters did not bind their own parameter positions", page, err)
	}
}

func waitForMetadataLock(t *testing.T, s *Store, done <-chan error, query string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatal("metadata operation crossed an uncommitted write", err)
		default:
		}
		var blocked bool
		if err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE query=$1 AND wait_event_type='Lock')", query).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("metadata operation never waited for the conflicting transaction")
}

func TestBackupVerificationDecryptsRecordsWithoutInitializing(t *testing.T) {
	dir := t.TempDir()
	dsn := testpg.DSN(t, dir)
	st, err := Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.SaveSource(model.Source{ID: "backup", Password: "private"}); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyBackup(context.Background(), dir, dsn)
	if err != nil || report["sources"] != 1 {
		t.Fatal(report, err)
	}
	if _, err = st.DB.Exec("UPDATE sources SET value='corrupted' WHERE id='backup'"); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyBackup(context.Background(), dir, dsn); err == nil {
		t.Fatal("corrupted encrypted record passed verification")
	}
}
