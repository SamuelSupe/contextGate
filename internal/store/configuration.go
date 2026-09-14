package store

import (
	"encoding/json"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

func (s *Store) CreateConfigurationAgent(name string, expires time.Time) (model.ConfigurationAgent, string, error) {
	a := model.ConfigurationAgent{ID: "cfg_" + secure.Random(16), Name: name, CreatedAt: time.Now().UTC(), ExpiresAt: expires.UTC()}
	token := "cfg_token_" + secure.Random(32)
	b, err := json.Marshal(a)
	if err == nil {
		_, err = s.DB.Exec("INSERT INTO configuration_agents(id,value,token_hash) VALUES($1,$2,$3)", a.ID, string(b), secure.Hash(token))
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
	b, err := json.Marshal(a)
	if err == nil {
		_, err = s.DB.Exec("UPDATE configuration_agents SET value=$1,token_hash=NULL WHERE id=$2", string(b), id)
	}
	return err
}
