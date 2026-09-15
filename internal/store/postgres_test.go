package store

import (
	"context"
	"encoding/json"
	"github.com/SamuelSupe/contextGate/internal/secure"
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
	if _, err := tx.Exec("UPDATE administrators SET password_hash='new-hash' WHERE username='admin'"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("DELETE FROM sessions"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Session("late-login", "csrf", time.Now().Add(time.Hour), "old-hash") }()
	waitForMetadataLock(t, s, done, "SELECT "+administratorColumns+" FROM administrators WHERE id=$1 FOR UPDATE")
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

func TestAdministratorMigrationRollsBackAndPreservesLegacyData(t *testing.T) {
	dir := t.TempDir()
	dsn := testpg.DSN(t, dir)
	s, err := Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	hash := secure.Password("legacy-admin-password")
	if err = s.SaveSource(model.Source{ID: "retained", Password: "database-secret"}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveAgent(model.Agent{ID: "query", Enabled: true, Sources: []string{"retained"}}, "query-token"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`DELETE FROM kv WHERE key='administrator_accounts_v1'; INSERT INTO sessions(hash,csrf,expires) VALUES('legacy-session','csrf',9999999999); INSERT INTO audit(at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code) VALUES(1,'admin','retained','source.update','',0,0,'')`); err != nil {
		t.Fatal(err)
	}
	if err = s.Set("admin_password", hash); err != nil {
		t.Fatal(err)
	}
	legacy := model.ConfigurationAgent{ID: "cfg_legacy", Name: "Legacy", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(legacy)
	if _, err = s.DB.Exec("INSERT INTO configuration_agents(id,value,token_hash) VALUES($1,$2,$3)", legacy.ID, string(raw), secure.Hash("legacy-token")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`CREATE FUNCTION fail_migration() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'migration fault'; END $$; CREATE TRIGGER fail_migration BEFORE DELETE ON sessions FOR EACH STATEMENT EXECUTE FUNCTION fail_migration()`); err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(dir, dsn); err == nil {
		opened.Close()
		t.Fatal("migration ignored storage failure")
	}
	if initialized, _ := s.AdministratorInitialized(); initialized {
		t.Fatal("failed migration left a partial account")
	}
	if kept, _ := s.Get("admin_password"); kept != hash {
		t.Fatal("failed migration lost legacy password")
	}
	if _, err = s.ConfigurationToken("legacy-token"); err != nil {
		t.Fatal("failed migration partially revoked token", err)
	}
	if _, err = s.DB.Exec("DROP TRIGGER fail_migration ON sessions; DROP FUNCTION fail_migration()"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		migrated, err := Open(dir, dsn)
		if err != nil {
			t.Fatal(err)
		}
		admin, err := migrated.AdministratorByUsername("ADMIN")
		if err != nil || admin.PasswordHash != hash || !secure.CheckPassword(admin.PasswordHash, "legacy-admin-password") || admin.Role != model.RoleSuperAdministrator {
			t.Fatal("legacy administrator password or role lost", err)
		}
		var sessions int
		migrated.DB.QueryRow("SELECT count(*) FROM sessions").Scan(&sessions)
		if sessions != 0 {
			t.Fatal("legacy browser session survived")
		}
		if _, err = migrated.ConfigurationToken("legacy-token"); err == nil {
			t.Fatal("legacy configuration token survived")
		}
		if identity, err := migrated.ConfigurationAgent("cfg_legacy"); err != nil || identity.AdministratorID != "" || identity.RevokedAt == nil {
			t.Fatal("legacy identity history lost or falsely attributed", err)
		}
		if _, err = migrated.TokenAgent("query-token"); err != nil {
			t.Fatal("query Agent grant lost", err)
		}
		if source, err := migrated.Source("retained"); err != nil || source.Password != "database-secret" {
			t.Fatal("encrypted source did not survive", err)
		}
		audit, err := migrated.Audits(AuditFilter{}, 100)
		if err != nil || len(audit) != 1 || audit[0].AdministratorID != "" || audit[0].ActorType != "legacy" {
			t.Fatal("historical administrator was incorrectly attributed", err)
		}
		identities, err := migrated.ConfigurationAgents()
		if err != nil || len(identities) != 2 {
			t.Fatal("migration not idempotent", err)
		}
		migrated.Close()
	}
}

func TestLastSuperAdministratorProtectedAcrossConcurrentStores(t *testing.T) {
	dir := t.TempDir()
	dsn := testpg.DSN(t, dir)
	s, err := Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.SetupAdministrator("first", "First", "hash")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateAdministrator(context.Background(), "second", "Second", model.RoleSuperAdministrator, "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	start := make(chan struct{})
	done := make(chan error, 2)
	for i, a := range []model.Administrator{first, second} {
		target := s
		if i == 1 {
			target = other
		}
		go func() {
			<-start
			_, _, err := target.UpdateAdministrator(context.Background(), a.ID, a.DisplayName, model.RoleAdministrator, false, a.Revision)
			done <- err
		}()
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		err := <-done
		if err == nil {
			success++
		} else if model.ErrorCode(err) == "conflict" {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM administrators WHERE enabled AND role='super_admin'").Scan(&count)
	if success != 1 || conflict != 1 || count != 1 {
		t.Fatal("concurrent account edits removed the last super administrator")
	}
}
