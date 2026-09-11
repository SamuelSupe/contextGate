package store

import (
	"database/sql"
	"encoding/json"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	"path/filepath"
	"testing"
	"time"
)

func TestUpgradeRetiresPreviouslyRevokedCredentials(t *testing.T) {
	dir := t.TempDir()
	if _, err := secure.OpenVault(dir); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE agents(id TEXT PRIMARY KEY,value TEXT NOT NULL,token_hash TEXT UNIQUE)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		id := "revoked"
		if enabled {
			id = "active"
		}
		b, _ := json.Marshal(model.Agent{ID: id, Enabled: enabled, AuthType: "token", ExpiresAt: time.Now().Add(time.Hour)})
		if _, err = db.Exec("INSERT INTO agents VALUES(?,?,?)", id, string(b), secure.Hash(id+"-token")); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.Agent("revoked")
	if err != nil || a.RevokedAt == nil {
		t.Fatal("legacy revocation was not preserved", err)
	}
	if _, err = st.TokenAgent("revoked-token"); err == nil {
		t.Fatal("legacy token hash retained")
	}
	active, err := st.TokenAgent("active-token")
	if err != nil || !active.Enabled {
		t.Fatal("active credential changed", err)
	}
	active.Enabled = false
	if err = st.SaveAgent(active, ""); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	paused, err := st.TokenAgent("active-token")
	if err != nil || paused.RevokedAt != nil {
		t.Fatal("restart turned a new pause into permanent revocation", err)
	}
}

func TestUpgradeCleansDanglingGrantsWithoutRetiringActiveTokens(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SaveSource(model.Source{ID: "kept", Name: "Kept source"}); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveAgent(model.Agent{ID: "reader", Name: "Reader", Enabled: true, Sources: []string{"kept", "deleted"}, ExpiresAt: time.Now().Add(time.Hour)}, "original-token"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB.Exec("DELETE FROM kv WHERE key='product_lifecycle_v2'"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.TokenAgent("original-token")
	if err != nil || !a.Enabled || a.RevokedAt != nil || len(a.Sources) != 1 || a.Sources[0] != "kept" || a.Revision == 0 {
		t.Fatal("upgrade changed active credentials or retained deleted grants", err)
	}
	revision := a.Revision
	st.Close()
	st, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, err = st.TokenAgent("original-token")
	if err != nil || a.Revision != revision {
		t.Fatal("upgrade repeated on restart", err)
	}
}
