package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/semantic"
)

func (s *Store) migrateSemantics() error {
	if _, err := s.Get("semantics_v1"); err == nil {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE semantics_state(source_id TEXT PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,revision INTEGER NOT NULL,value TEXT NOT NULL)`,
		`CREATE TABLE semantics_entries(source_id TEXT NOT NULL REFERENCES semantics_state(source_id) ON DELETE CASCADE,phase TEXT NOT NULL,id TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(source_id,phase,id))`,
		`CREATE TABLE semantics_evidence(source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,definition TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(source_id,definition))`,
		`ALTER TABLE audit ADD COLUMN template_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE audit ADD COLUMN template_version TEXT NOT NULL DEFAULT ''`,
		`INSERT INTO kv(key,value) VALUES('semantics_v1','1')`,
	} {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Semantics(source string) (semantic.State, error) {
	st := semantic.State{Draft: semantic.Empty(), Published: semantic.Empty()}
	tx, err := s.DB.Begin()
	if err != nil {
		return st, err
	}
	defer tx.Rollback()
	var sealed string
	err = tx.QueryRow("SELECT value FROM semantics_state WHERE source_id=?", source).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	b, err := s.Vault.Open(sealed, "semantics-state:"+source)
	if err != nil {
		return st, err
	}
	if err = json.Unmarshal(b, &st); err != nil {
		return st, err
	}
	st.Draft.Entries, st.Published.Entries = []semantic.Entry{}, []semantic.Entry{}
	rows, err := tx.Query("SELECT phase,id,value FROM semantics_entries WHERE source_id=? ORDER BY phase,id LIMIT ?", source, 2*semantic.MaxEntries+1)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		if count > 2*semantic.MaxEntries {
			return st, errors.New("semantic storage limit exceeded")
		}
		var phase, id, value string
		if err = rows.Scan(&phase, &id, &value); err != nil {
			return st, err
		}
		b, err = s.Vault.Open(value, "semantics-entry:"+source+":"+phase+":"+id)
		if err != nil {
			return st, err
		}
		var en semantic.Entry
		if err = json.Unmarshal(b, &en); err != nil {
			return st, err
		}
		if phase == "draft" {
			st.Draft.Entries = append(st.Draft.Entries, en)
		} else {
			st.Published.Entries = append(st.Published.Entries, en)
		}
	}
	return st, rows.Err()
}

// WriteSemantics performs a compare-and-swap and replaces both snapshots in one
// transaction. Callers serialize source/semantic mutations with Mutations.
func (s *Store) WriteSemantics(source string, expected int64, st semantic.State, evidence ...semantic.Evidence) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st.Revision = expected + 1
	meta := st
	meta.Draft.Entries, meta.Published.Entries = nil, nil
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	sealed := s.Vault.Seal(b, "semantics-state:"+source)
	var res sql.Result
	if expected == 0 {
		res, err = tx.Exec("INSERT OR IGNORE INTO semantics_state(source_id,revision,value) VALUES(?,?,?)", source, st.Revision, sealed)
	} else {
		res, err = tx.Exec("UPDATE semantics_state SET revision=?,value=? WHERE source_id=? AND revision=?", st.Revision, sealed, source, expected)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return model.Fail("conflict", "Semantic draft changed; reload before saving or publishing")
	}
	if _, err = tx.Exec("DELETE FROM semantics_entries WHERE source_id=?", source); err != nil {
		return err
	}
	for phase, snapshot := range map[string]semantic.Snapshot{"draft": st.Draft, "published": st.Published} {
		for _, en := range snapshot.Entries {
			b, err = json.Marshal(en)
			if err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT INTO semantics_entries(source_id,phase,id,value) VALUES(?,?,?,?)", source, phase, en.ID, s.Vault.Seal(b, "semantics-entry:"+source+":"+phase+":"+en.ID)); err != nil {
				return err
			}
		}
	}
	for _, ev := range evidence {
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO semantics_evidence(source_id,definition,value) VALUES(?,?,?) ON CONFLICT(source_id,definition) DO UPDATE SET value=excluded.value", source, ev.Definition, s.Vault.Seal(b, "semantics-evidence:"+source+":"+ev.Definition)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SemanticEvidence(source, definition string) (semantic.Evidence, error) {
	var ev semantic.Evidence
	var value string
	err := s.DB.QueryRow("SELECT value FROM semantics_evidence WHERE source_id=? AND definition=?", source, definition).Scan(&value)
	if err != nil {
		return ev, err
	}
	b, err := s.Vault.Open(value, "semantics-evidence:"+source+":"+definition)
	if err != nil {
		return ev, err
	}
	err = json.Unmarshal(b, &ev)
	return ev, err
}

func (s *Store) SaveSemanticEvidence(source string, ev semantic.Evidence) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO semantics_evidence(source_id,definition,value) VALUES(?,?,?) ON CONFLICT(source_id,definition) DO UPDATE SET value=excluded.value", source, ev.Definition, s.Vault.Seal(b, "semantics-evidence:"+source+":"+ev.Definition))
	if err != nil {
		return fmt.Errorf("save template evidence: %w", err)
	}
	return nil
}
