package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type Store struct {
	DB        *sql.DB
	Vault     *secure.Vault
	Mutations sync.Mutex
}

func Open(dir string) (*Store, error) {
	vault, e := secure.OpenVault(dir)
	if e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "hub.db")
	db, e := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS sources(id TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS agents(id TEXT PRIMARY KEY,value TEXT NOT NULL,token_hash TEXT UNIQUE);
 CREATE TABLE IF NOT EXISTS sessions(hash TEXT PRIMARY KEY,csrf TEXT NOT NULL,expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY AUTOINCREMENT,at INTEGER NOT NULL,agent_id TEXT NOT NULL,source_id TEXT NOT NULL,operation TEXT NOT NULL,fingerprint TEXT NOT NULL,elapsed_ms INTEGER NOT NULL,rows INTEGER NOT NULL,error_code TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS audit_at ON audit(at);
 CREATE TABLE IF NOT EXISTS oauth(kind TEXT NOT NULL,id TEXT NOT NULL,value TEXT NOT NULL,expires INTEGER NOT NULL,request_id TEXT NOT NULL DEFAULT '',active INTEGER NOT NULL DEFAULT 1,PRIMARY KEY(kind,id));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		db.Close()
		return nil, e
	}
	st := &Store{DB: db, Vault: vault}
	if e = st.migrate(); e != nil {
		db.Close()
		return nil, e
	}
	if e = st.migrateSemantics(); e != nil {
		db.Close()
		return nil, e
	}
	if e = st.migrateOntologies(); e != nil {
		db.Close()
		return nil, e
	}
	return st, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Get(key string) (string, error) {
	var v string
	e := s.DB.QueryRow("SELECT value FROM kv WHERE key=?", key).Scan(&v)
	return v, e
}
func (s *Store) Set(key, value string) error {
	_, e := s.DB.Exec("INSERT INTO kv(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return e
}
func (s *Store) Setup(passwordHash string) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO kv(key,value) VALUES('admin_password',?)", passwordHash)
	if e != nil {
		return errors.New("already initialized")
	}
	_, e = tx.Exec("DELETE FROM kv WHERE key='setup_hash'")
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Source(id string) (model.Source, error) {
	var src model.Source
	var v string
	e := s.DB.QueryRow("SELECT value FROM sources WHERE id=?", id).Scan(&v)
	if e != nil {
		return src, e
	}
	b, e := s.Vault.Open(v, "source:"+id)
	if e != nil {
		return src, e
	}
	e = json.Unmarshal(b, &src)
	return src, e
}
func (s *Store) Sources() ([]model.Source, error) {
	rows, e := s.DB.Query("SELECT id,value FROM sources ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Source{}
	for rows.Next() {
		var id, v string
		if e = rows.Scan(&id, &v); e != nil {
			return nil, e
		}
		b, e := s.Vault.Open(v, "source:"+id)
		if e != nil {
			return nil, e
		}
		var src model.Source
		if e = json.Unmarshal(b, &src); e != nil {
			return nil, e
		}
		out = append(out, src)
	}
	return out, rows.Err()
}
func (s *Store) SaveSource(src model.Source) error {
	b, e := json.Marshal(src)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec("INSERT INTO sources(id,value) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", src.ID, s.Vault.Seal(b, "source:"+src.ID))
	return e
}
func (s *Store) DeleteSource(id string) error {
	agents, err := s.Agents()
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM sources WHERE id=?", id); err != nil {
		return err
	}
	for _, a := range agents {
		if !slices.Contains(a.Sources, id) {
			continue
		}
		a.Sources = slices.DeleteFunc(a.Sources, func(source string) bool { return source == id })
		a.Revision++
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE agents SET value=? WHERE id=?", string(b), a.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Agent(id string) (model.Agent, error) {
	var a model.Agent
	var v string
	e := s.DB.QueryRow("SELECT value FROM agents WHERE id=?", id).Scan(&v)
	if e == nil {
		e = json.Unmarshal([]byte(v), &a)
	}
	if a.Sources == nil {
		a.Sources = []string{}
	}
	return a, e
}
func (s *Store) TokenAgent(token string) (model.Agent, error) {
	var a model.Agent
	var v string
	e := s.DB.QueryRow("SELECT value FROM agents WHERE token_hash=?", secure.Hash(token)).Scan(&v)
	if e == nil {
		e = json.Unmarshal([]byte(v), &a)
	}
	if a.Sources == nil {
		a.Sources = []string{}
	}
	return a, e
}
func (s *Store) Agents() ([]model.Agent, error) {
	rows, e := s.DB.Query("SELECT value FROM agents ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Agent{}
	for rows.Next() {
		var v string
		var a model.Agent
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(v), &a); e != nil {
			return nil, e
		}
		if a.Sources == nil {
			a.Sources = []string{}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) SaveAgent(a model.Agent, token string) error {
	if a.Sources == nil {
		a.Sources = []string{}
	}
	b, e := json.Marshal(a)
	if e != nil {
		return e
	}
	if token == "" {
		_, e = s.DB.Exec("INSERT INTO agents(id,value) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", a.ID, string(b))
	} else {
		_, e = s.DB.Exec("INSERT INTO agents(id,value,token_hash) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value,token_hash=excluded.token_hash", a.ID, string(b), secure.Hash(token))
	}
	return e
}
func (s *Store) Session(token, csrf string, expires time.Time) error {
	_, e := s.DB.Exec("INSERT INTO sessions(hash,csrf,expires) VALUES(?,?,?)", secure.Hash(token), csrf, expires.Unix())
	return e
}
func (s *Store) CheckSession(token string) (string, error) {
	var csrf string
	e := s.DB.QueryRow("SELECT csrf FROM sessions WHERE hash=? AND expires>?", secure.Hash(token), time.Now().Unix()).Scan(&csrf)
	return csrf, e
}
func (s *Store) DeleteSession(token string) error {
	_, e := s.DB.Exec("DELETE FROM sessions WHERE hash=?", secure.Hash(token))
	return e
}
func (s *Store) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.DB.Exec("DELETE FROM sessions WHERE expires<?", time.Now().Unix())
			s.DB.Exec("DELETE FROM audit WHERE at<?", time.Now().Add(-30*24*time.Hour).UnixMilli())
			s.DB.Exec("DELETE FROM oauth WHERE expires>0 AND expires<?", time.Now().Unix())
		}
	}
}
