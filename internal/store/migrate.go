package store

import (
	"encoding/json"
	"slices"
	"time"
)

func (s *Store) migrate() error {
	if err := s.migrateLifecycleV1(); err != nil {
		return err
	}
	if _, err := s.Get("product_lifecycle_v2"); err == nil {
		return nil
	}
	agents, err := s.Agents()
	if err != nil {
		return err
	}
	sources, err := s.Sources()
	if err != nil {
		return err
	}
	ids := make(map[string]bool, len(sources))
	for _, source := range sources {
		ids[source.ID] = true
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range agents {
		a.Sources = slices.DeleteFunc(a.Sources, func(id string) bool { return !ids[id] })
		a.Revision++
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE agents SET value=? WHERE id=?", string(b), a.ID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO kv(key,value) VALUES('product_lifecycle_v2','1')"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) migrateLifecycleV1() error {
	if _, err := s.Get("product_lifecycle_v1"); err == nil {
		return nil
	}
	agents, err := s.Agents()
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"ALTER TABLE audit ADD COLUMN request_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE audit ADD COLUMN native_code TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE audit ADD COLUMN preview INTEGER NOT NULL DEFAULT 0",
		"CREATE INDEX audit_agent ON audit(agent_id,id)",
	} {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	// Before pause was introduced, every disabled Agent was presented as revoked.
	// Retire those credentials permanently instead of allowing a later edit to revive them.
	now := time.Now()
	for _, a := range agents {
		if a.Enabled {
			continue
		}
		a.RevokedAt = &now
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE agents SET value=?,token_hash=NULL WHERE id=?", string(b), a.ID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO kv(key,value) VALUES('product_lifecycle_v1','1')"); err != nil {
		return err
	}
	return tx.Commit()
}
