package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

type SemanticVersion struct {
	Version     int64              `json:"version,string"`
	PublishedAt *time.Time         `json:"published_at,omitempty"`
	EntryCount  int                `json:"entry_count"`
	Snapshot    *semantic.Snapshot `json:"snapshot,omitempty"`
}

func (s *Store) archiveSemantics(tx *sql.Tx, source string, version int64, snapshot semantic.Snapshot) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var ontologyID, ontologyVersion any
	if snapshot.Ontology != nil {
		ontologyID, ontologyVersion = snapshot.Ontology.OntologyID, snapshot.Ontology.Version
	}
	_, err = tx.Exec(`INSERT INTO semantics_versions(source_id,version,published_at,entry_count,ontology_id,ontology_version,value)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(source_id,version) DO NOTHING`, source, version, time.Now().UnixMilli(), len(snapshot.Entries), ontologyID, ontologyVersion,
		s.Vault.Seal(b, "semantics-version:"+source+":"+strconv.FormatInt(version, 10)))
	return err
}

// Only the current snapshot can be retained when upgrading an older store.
func (s *Store) retainCurrentSemantics() error {
	rows, err := s.DB.Query(`SELECT source_id FROM semantics_state WHERE source_id NOT IN (SELECT source_id FROM semantics_versions)`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		st, err := s.Semantics(id)
		if err != nil {
			return err
		}
		if st.PublishedVersion == 0 {
			continue
		}
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		if err = s.archiveSemantics(tx, id, st.PublishedVersion, st.Published); err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec("UPDATE semantics_versions SET published_at=0 WHERE source_id=$1 AND version=$2", id, st.PublishedVersion); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SemanticVersions(source string, before int64) ([]SemanticVersion, error) {
	if before <= 0 {
		before = 1<<63 - 1
	}
	rows, err := s.DB.Query("SELECT version,published_at,entry_count FROM semantics_versions WHERE source_id=$1 AND version<$2 ORDER BY version DESC LIMIT 25", source, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SemanticVersion{}
	for rows.Next() {
		var v SemanticVersion
		var at int64
		if err = rows.Scan(&v.Version, &at, &v.EntryCount); err != nil {
			return nil, err
		}
		if at > 0 {
			when := time.UnixMilli(at)
			v.PublishedAt = &when
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SemanticVersion(source string, version int64) (SemanticVersion, error) {
	v := SemanticVersion{Version: version}
	var sealed string
	var at int64
	err := s.DB.QueryRow("SELECT published_at,entry_count,value FROM semantics_versions WHERE source_id=$1 AND version=$2", source, version).Scan(&at, &v.EntryCount, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return v, model.Fail("not_found", "Semantic version not found")
	}
	if err != nil {
		return v, err
	}
	b, err := s.Vault.Open(sealed, "semantics-version:"+source+":"+strconv.FormatInt(version, 10))
	if err != nil {
		return v, err
	}
	if at > 0 {
		when := time.UnixMilli(at)
		v.PublishedAt = &when
	}
	v.Snapshot = &semantic.Snapshot{}
	err = json.Unmarshal(b, v.Snapshot)
	return v, err
}
