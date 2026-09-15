package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

func (s *Store) CreateConfigurationAgent(name string, expires time.Time) (model.ConfigurationAgent, string, error) {
	a, err := s.SoleAdministrator("")
	if err != nil {
		return model.ConfigurationAgent{}, "", err
	}
	return s.IssueConfigurationToken(context.Background(), a.ID, name, expires, 0)
}
func (s *Store) AdministratorConfiguration(id string) (model.ConfigurationAgent, error) {
	var raw string
	err := s.DB.QueryRow("SELECT value FROM configuration_agents WHERE administrator_id=$1", id).Scan(&raw)
	var a model.ConfigurationAgent
	if err == nil {
		err = json.Unmarshal([]byte(raw), &a)
	}
	return a, err
}
func (s *Store) IssueConfigurationToken(ctx context.Context, owner, name string, expires time.Time, revision int64) (model.ConfigurationAgent, string, error) {
	s.Mutations.Lock()
	defer s.Mutations.Unlock()
	if err := model.CheckConfigurationContext(ctx); err != nil {
		return model.ConfigurationAgent{}, "", err
	}
	administrator, err := s.Administrator(owner)
	if err != nil || !administrator.Active() || administrator.MustChangePassword {
		return model.ConfigurationAgent{}, "", model.Fail("forbidden", "An active administrator with a permanent password is required")
	}
	a, err := s.AdministratorConfiguration(owner)
	if err != nil {
		return a, "", err
	}
	if revision > 0 && a.Revision != revision {
		return a, "", model.Fail("conflict", "Configuration identity changed; reload before issuing a token")
	}
	a.Revision++
	a.ExpiresAt = expires.UTC()
	a.RevokedAt = nil
	if name != "" {
		a.Name = name
	}
	token := "cfg_token_" + secure.Random(32)
	b, err := json.Marshal(a)
	if err == nil {
		_, err = s.DB.Exec("UPDATE configuration_agents SET value=$1,token_hash=$2 WHERE id=$3", string(b), secure.Hash(token), a.ID)
	}
	return a, token, err
}

func (s *Store) ConfigurationAgent(id string) (model.ConfigurationAgent, error) {
	var a model.ConfigurationAgent
	var raw string
	err := s.DB.QueryRow("SELECT value FROM configuration_agents WHERE id=$1", id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &a)
	}
	return a, err
}

func (s *Store) ConfigurationToken(token string) (model.ConfigurationAgent, error) {
	var a model.ConfigurationAgent
	var raw string
	err := s.DB.QueryRow("SELECT value FROM configuration_agents WHERE token_hash=$1", secure.Hash(token)).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &a)
	}
	return a, err
}

func (s *Store) ConfigurationAgents() ([]model.ConfigurationAgent, error) {
	rows, err := s.DB.Query("SELECT value FROM configuration_agents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ConfigurationAgent{}
	for rows.Next() {
		var raw string
		var a model.ConfigurationAgent
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) RevokeConfigurationAgent(id string) error {
	a, err := s.ConfigurationAgent(id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	a.RevokedAt = &now
	a.Revision++
	b, err := json.Marshal(a)
	if err == nil {
		_, err = s.DB.Exec("UPDATE configuration_agents SET value=$1,token_hash=NULL WHERE id=$2", string(b), id)
	}
	return err
}
