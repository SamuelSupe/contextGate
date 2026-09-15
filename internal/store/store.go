package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"slices"
	"sync"
	"time"
)

type Store struct {
	DB        *sql.DB
	Vault     *secure.Vault
	Mutations sync.Mutex
}

func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Get(key string) (string, error) {
	var v string
	e := s.DB.QueryRow("SELECT value FROM kv WHERE key=$1", key).Scan(&v)
	return v, e
}
func (s *Store) Set(key, value string) error {
	_, e := s.DB.Exec("INSERT INTO kv(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return e
}
func (s *Store) Setup(passwordHash string) error {
	_, err := s.SetupAdministrator("admin", "Administrator", passwordHash)
	return err
}
func (s *Store) Source(id string) (model.Source, error) {
	var src model.Source
	var v string
	e := s.DB.QueryRow("SELECT value FROM sources WHERE id=$1", id).Scan(&v)
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
	_, e = s.DB.Exec("INSERT INTO sources(id,value) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET value=excluded.value", src.ID, s.Vault.Seal(b, "source:"+src.ID))
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
	if _, err = tx.Exec("DELETE FROM sources WHERE id=$1", id); err != nil {
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
		if _, err = tx.Exec("UPDATE agents SET value=$1 WHERE id=$2", string(b), a.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Agent(id string) (model.Agent, error) {
	var a model.Agent
	var v string
	e := s.DB.QueryRow("SELECT value FROM agents WHERE id=$1", id).Scan(&v)
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
	e := s.DB.QueryRow("SELECT value FROM agents WHERE token_hash=$1", secure.Hash(token)).Scan(&v)
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
		_, e = s.DB.Exec("INSERT INTO agents(id,value) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET value=excluded.value", a.ID, string(b))
	} else {
		_, e = s.DB.Exec("INSERT INTO agents(id,value,token_hash) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET value=excluded.value,token_hash=excluded.token_hash", a.ID, string(b), secure.Hash(token))
	}
	return e
}

func (s *Store) Session(token, csrf string, expires time.Time, passwordHash string) error {
	a, err := s.SoleAdministrator("")
	if err != nil {
		return err
	}
	return s.CreateAdministratorSession(a.ID, token, csrf, expires, passwordHash)
}
func (s *Store) CheckSession(token string) (string, error) {
	_, csrf, err := s.AdministratorSession(token)
	return csrf, err
}
func (s *Store) DeleteSession(token string) error {
	_, e := s.DB.Exec("DELETE FROM sessions WHERE hash=$1", secure.Hash(token))
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
			s.DB.Exec("DELETE FROM sessions WHERE expires<$1", time.Now().Unix())
			s.DB.Exec("DELETE FROM audit WHERE at<$1", time.Now().Add(-30*24*time.Hour).UnixMilli())
			s.DB.Exec("DELETE FROM oauth WHERE expires>0 AND expires<$1", time.Now().Unix())
		}
	}
}
