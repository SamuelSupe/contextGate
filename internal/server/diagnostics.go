package server

import (
	"context"
	"net/http"
	"time"

	"github.com/SamuelSupe/contextGate/internal/version"
)

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	checks := []map[string]any{}
	var pgVersion string
	var tls bool
	err := s.Store.DB.QueryRowContext(ctx, `SELECT current_setting('server_version'),coalesce((SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid()),false)`).Scan(&pgVersion, &tls)
	if err != nil {
		checks = append(checks, map[string]any{"check": "metadata_database", "status": "failed"})
	} else {
		checks = append(checks, map[string]any{"check": "metadata_database", "status": "passed", "server_version": pgVersion, "tls": tls})
	}
	status := "failed"
	var sealed string
	err = s.Store.DB.QueryRowContext(ctx, "SELECT value FROM kv WHERE key='master_key_check'").Scan(&sealed)
	if err == nil {
		if plain, e := s.Store.Vault.Open(sealed, "master-key-check"); e == nil && string(plain) == "mcpdbhub" {
			status = "passed"
		}
	}
	checks = append(checks, map[string]any{"check": "encryption_key", "status": status})
	var sources, agents, history int
	err = s.Store.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM sources),(SELECT count(*) FROM agents),(SELECT count(*) FROM semantics_versions)`).Scan(&sources, &agents, &history)
	if err == nil {
		checks = append(checks, map[string]any{"check": "metadata_schema", "status": "passed", "sources": sources, "agents": agents, "semantic_versions": history})
	} else {
		checks = append(checks, map[string]any{"check": "metadata_schema", "status": "failed"})
	}
	stats := s.Store.DB.Stats()
	write(w, 200, map[string]any{"checked_at": time.Now().UTC(), "version": version.Version, "commit": version.BuildCommit(), "storage": "postgresql", "checks": checks,
		"connection_pool": map[string]any{"open": stats.OpenConnections, "in_use": stats.InUse, "idle": stats.Idle, "max": stats.MaxOpenConnections, "wait_count": stats.WaitCount},
		"backup":          map[string]any{"status": "not_monitored", "guide": "docs/operations.md", "required_components": []string{"PostgreSQL dump", "matching encryption key"}},
	})
}
