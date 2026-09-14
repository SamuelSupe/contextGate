package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// VerifyBackup never initializes a schema, exports plaintext or opens a source.
// It is also safe to use against a restored database with outbound traffic denied.
func VerifyBackup(ctx context.Context, dir, databaseURL string) (map[string]int, error) {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil || databaseURL == "" {
		return nil, errors.New("valid MCPDBHUB_DATABASE_URL is required")
	}
	cfg.ConnectTimeout = 10 * time.Second
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, errors.New("cannot connect to restored PostgreSQL metadata")
	}
	defer tx.Rollback()
	vault, err := secure.OpenVault(dir, true)
	if err != nil {
		return nil, errors.New("cannot open backup encryption key")
	}
	var marker string
	if err = tx.QueryRowContext(ctx, "SELECT value FROM kv WHERE key='master_key_check'").Scan(&marker); err != nil {
		return nil, errors.New("metadata key check missing from restored database")
	}
	if plain, err := vault.Open(marker, "master-key-check"); err != nil || string(plain) != "mcpdbhub" {
		return nil, errors.New("backup encryption key does not match restored metadata")
	}
	rows, err := tx.QueryContext(ctx, `SELECT 'sources',value,'source:'||id FROM sources
 UNION ALL SELECT 'semantics',value,'semantics-state:'||source_id FROM semantics_state
 UNION ALL SELECT 'entries',value,'semantics-entry:'||source_id||':'||phase||':'||id FROM semantics_entries
 UNION ALL SELECT 'evidence',value,'semantics-evidence:'||source_id||':'||definition FROM semantics_evidence
 UNION ALL SELECT 'semantic_versions',value,'semantics-version:'||source_id||':'||version::text FROM semantics_versions
 UNION ALL SELECT 'ontologies',value,'ontology:'||id FROM ontologies
 UNION ALL SELECT 'ontology_versions',value,'ontology-version:'||ontology_id||':'||version::text FROM ontology_versions
 UNION ALL SELECT 'oauth',value,'oauth:'||kind||':'||id FROM oauth
 UNION ALL SELECT 'evaluation_questions',value,'evaluation_questions:'||source_id||':'||id FROM evaluation_questions
 UNION ALL SELECT 'evaluations',value,'evaluations:'||source_id||':'||id FROM evaluations
 UNION ALL SELECT 'health',value,'health:'||source_id FROM source_health
 UNION ALL SELECT 'audit_export',value,key FROM kv WHERE key='audit_otlp_v1'`)
	if err != nil {
		return nil, errors.New("restored metadata schema is incomplete or incompatible")
	}
	counts := map[string]int{}
	for rows.Next() {
		var table, sealed, aad string
		if err = rows.Scan(&table, &sealed, &aad); err != nil {
			rows.Close()
			return nil, errors.New("restored metadata could not be read")
		}
		plain, err := vault.Open(sealed, aad)
		if err != nil || !json.Valid(plain) {
			rows.Close()
			return nil, errors.New("an encrypted metadata record failed verification")
		}
		counts[table]++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, errors.New("restored metadata scan failed")
	}
	for _, table := range []string{"agents", "configuration_agents", "audit", "sessions"} {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			return nil, errors.New("restored metadata counts failed")
		}
		counts[table] = count
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("metadata verification transaction failed")
	}
	return counts, nil
}
