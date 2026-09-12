package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/ontology"
)

func (s *Store) migrateOntologies() error {
	if _, err := s.Get("ontologies_v1"); err == nil {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`CREATE TABLE ontologies(id TEXT PRIMARY KEY,revision INTEGER NOT NULL,value TEXT NOT NULL)`,
		`CREATE TABLE ontology_versions(ontology_id TEXT NOT NULL REFERENCES ontologies(id) ON DELETE CASCADE,version INTEGER NOT NULL,value TEXT NOT NULL,PRIMARY KEY(ontology_id,version))`,
		`CREATE TABLE ontology_bindings(source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,phase TEXT NOT NULL,ontology_id TEXT NOT NULL,version INTEGER NOT NULL,PRIMARY KEY(source_id,phase))`,
		`CREATE INDEX ontology_binding_reference ON ontology_bindings(ontology_id,version)`,
		`ALTER TABLE audit ADD COLUMN ontology_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE audit ADD COLUMN ontology_version TEXT NOT NULL DEFAULT ''`,
		`INSERT INTO kv(key,value) VALUES('ontologies_v1','1')`,
	} {
		if _, err = tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Ontology(id string) (ontology.State, error) {
	var out ontology.State
	var sealed string
	err := s.DB.QueryRow("SELECT value FROM ontologies WHERE id=?", id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return out, model.Fail("not_found", "Ontology not found")
	}
	if err != nil {
		return out, err
	}
	b, err := s.Vault.Open(sealed, "ontology:"+id)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(b, &out)
	ontology.NormalizeDefinition(&out.Draft)
	return out, err
}

func (s *Store) Ontologies() ([]ontology.State, error) {
	rows, err := s.DB.Query("SELECT id,value FROM ontologies ORDER BY id LIMIT 201")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ontology.State{}
	for rows.Next() {
		var id, sealed string
		if err = rows.Scan(&id, &sealed); err != nil {
			return nil, err
		}
		b, err := s.Vault.Open(sealed, "ontology:"+id)
		if err != nil {
			return nil, err
		}
		var st ontology.State
		if err = json.Unmarshal(b, &st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) OntologyVersion(id string, version int64) (ontology.Version, error) {
	var out ontology.Version
	var sealed string
	err := s.DB.QueryRow("SELECT value FROM ontology_versions WHERE ontology_id=? AND version=?", id, version).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return out, model.Fail("not_found", "Ontology version not found; repair the draft binding")
	}
	if err != nil {
		return out, err
	}
	b, err := s.Vault.Open(sealed, "ontology-version:"+id+":"+strconv.FormatInt(version, 10))
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(b, &out)
	ontology.NormalizeDefinition(&out.Definition)
	return out, err
}

func (s *Store) OntologyVersions(id string, before int64) ([]string, error) {
	if before <= 0 {
		before = 1<<63 - 1
	}
	rows, err := s.DB.Query("SELECT version FROM ontology_versions WHERE ontology_id=? AND version<? ORDER BY version DESC LIMIT 100", id, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v int64
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, strconv.FormatInt(v, 10))
	}
	return out, rows.Err()
}

// Callers hold Mutations, including when checking archive status and bindings.
func (s *Store) WriteOntology(st ontology.State, expected int64, publish bool) error {
	ontology.NormalizeDefinition(&st.Draft)
	if err := ontology.Bounded(st.Draft); err != nil {
		return model.Fail("invalid_ontology", err.Error())
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st.Revision = expected + 1
	if publish {
		st.LatestVersion++
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	sealed := s.Vault.Seal(b, "ontology:"+st.ID)
	var result sql.Result
	if expected == 0 {
		var count int
		if err = tx.QueryRow("SELECT count(*) FROM ontologies").Scan(&count); err != nil {
			return err
		}
		if count >= 200 {
			return model.Fail("limit_exceeded", "At most 200 ontologies are supported")
		}
		result, err = tx.Exec("INSERT OR IGNORE INTO ontologies(id,revision,value) VALUES(?,?,?)", st.ID, st.Revision, sealed)
	} else {
		result, err = tx.Exec("UPDATE ontologies SET revision=?,value=? WHERE id=? AND revision=?", st.Revision, sealed, st.ID, expected)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return model.Fail("conflict", "Ontology draft changed; reload before saving or publishing")
	}
	if publish {
		if err = ontology.Validate(st.Draft); err != nil {
			return model.Fail("invalid_ontology", err.Error())
		}
		v := ontology.Version{OntologyID: st.ID, Version: st.LatestVersion, PublishedAt: time.Now().UTC(), Definition: st.Draft}
		b, err = json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO ontology_versions(ontology_id,version,value) VALUES(?,?,?)", st.ID, v.Version, s.Vault.Seal(b, "ontology-version:"+st.ID+":"+strconv.FormatInt(v.Version, 10)))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

type OntologyUsage struct {
	SourceID string `json:"source_id"`
	Phase    string `json:"phase"`
	Version  int64  `json:"version,string"`
}

func (s *Store) OntologyUsage(id string) ([]OntologyUsage, error) {
	rows, err := s.DB.Query("SELECT source_id,phase,version FROM ontology_bindings WHERE ontology_id=? ORDER BY source_id,phase", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OntologyUsage{}
	for rows.Next() {
		var u OntologyUsage
		if err = rows.Scan(&u.SourceID, &u.Phase, &u.Version); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) DeleteOntologyVersion(id string, version, revision int64) error {
	st, err := s.Ontology(id)
	if err != nil {
		return err
	}
	if st.Revision != revision {
		return model.Fail("conflict", "Ontology changed; reload before deleting")
	}
	var count int
	q := "SELECT count(*) FROM ontology_bindings WHERE ontology_id=?"
	args := []any{id}
	if version > 0 {
		q += " AND version=?"
		args = append(args, version)
	}
	if err = s.DB.QueryRow(q, args...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return model.Fail("conflict", "Referenced ontologies and versions cannot be deleted; remove draft and published bindings first")
	}
	if version == 0 {
		_, err = s.DB.Exec("DELETE FROM ontologies WHERE id=?", id)
		return err
	}
	if version == st.LatestVersion {
		return model.Fail("conflict", "The latest version is the draft discard target; archive or delete an unreferenced ontology instead")
	}
	_, err = s.DB.Exec("DELETE FROM ontology_versions WHERE ontology_id=? AND version=?", id, version)
	return err
}
