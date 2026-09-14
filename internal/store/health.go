package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/SamuelSupe/contextGate/internal/model"
)

func (s *Store) HealthConfig() (model.HealthConfig, error) {
	cfg := model.HealthConfig{IntervalMinutes: 30}
	value, err := s.Get("health_config")
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal([]byte(value), &cfg)
	return cfg, err
}

func (s *Store) SaveHealthConfig(cfg model.HealthConfig) error {
	old, err := s.HealthConfig()
	if err != nil {
		return err
	}
	if cfg.Revision != old.Revision {
		return model.Fail("conflict", "Health settings changed; reload before saving")
	}
	if cfg.IntervalMinutes < 5 || cfg.IntervalMinutes > 1440 {
		return model.Fail("invalid_input", "Check interval must be 5–1440 minutes")
	}
	cfg.Revision++
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.Set("health_config", string(b))
}

// Hashes and object names are encrypted with the same key as source metadata.
type healthRecord struct {
	BaselineColumns map[string][]model.Column `json:"baseline_columns"`
	CurrentColumns  map[string][]model.Column `json:"current_columns"`
	model.SourceHealth
	Baseline map[string]string `json:"baseline"`
	Current  map[string]string `json:"current"`
}

func (s *Store) SourceHealth(source string) (model.SourceHealth, error) {
	var v healthRecord
	var sealed string
	err := s.DB.QueryRow("SELECT value FROM source_health WHERE source_id=$1", source).Scan(&sealed)
	if err != nil {
		return v.SourceHealth, err
	}
	b, err := s.Vault.Open(sealed, "health:"+source)
	if err != nil {
		return v.SourceHealth, err
	}
	err = json.Unmarshal(b, &v)
	v.SourceHealth.Baseline, v.SourceHealth.Current = v.Baseline, v.Current
	v.SourceHealth.BaselineColumns, v.SourceHealth.CurrentColumns = v.BaselineColumns, v.CurrentColumns
	return v.SourceHealth, err
}

func (s *Store) SaveSourceHealth(v model.SourceHealth) error {
	b, err := json.Marshal(healthRecord{SourceHealth: v, Baseline: v.Baseline, Current: v.Current, BaselineColumns: v.BaselineColumns, CurrentColumns: v.CurrentColumns})
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO source_health(source_id,value) VALUES($1,$2) ON CONFLICT(source_id) DO UPDATE SET value=excluded.value", v.SourceID, s.Vault.Seal(b, "health:"+v.SourceID))
	return err
}
