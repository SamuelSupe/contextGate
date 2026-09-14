package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
)

func (s *Server) healthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.requireAdmin(s.healthOverview))
	mux.HandleFunc("POST /api/health/{id}/check", s.requireAdmin(s.healthCheck))
	mux.HandleFunc("POST /api/health/{id}/baseline", s.requireAdmin(s.acceptHealthBaseline))
	mux.HandleFunc("GET /api/settings/health", s.requireAdmin(s.healthSettings))
	mux.HandleFunc("PUT /api/settings/health", s.requireAdmin(s.saveHealthSettings))
	mux.HandleFunc("GET /api/settings/diagnostics", s.requireAdmin(s.diagnostics))
}

func (s *Server) healthSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.HealthConfig()
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, cfg)
}
func (s *Server) saveHealthSettings(w http.ResponseWriter, r *http.Request) {
	var cfg model.HealthConfig
	if err := decode(r, &cfg); err != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid health settings"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := s.Store.SaveHealthConfig(cfg); err != nil {
		semanticFailure(w, err)
		return
	}
	s.healthSettings(w, r)
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	v, err := s.checkSourceHealth(r.Context(), r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) acceptHealthBaseline(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CheckedAt time.Time `json:"checked_at"`
	}
	if err := decode(r, &input); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid health baseline"))
		return
	}
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	src, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	v, err := s.Store.SourceHealth(src.ID)
	if err != nil {
		semanticFailure(w, model.Fail("not_found", "Run a health check before accepting a baseline"))
		return
	}
	if !v.CheckedAt.Equal(input.CheckedAt) || v.ConnectionRevision != src.ConnectionRevision || v.PublishedVersion != st.PublishedVersion {
		semanticFailure(w, model.Fail("conflict", "Health evidence changed; refresh before accepting"))
		return
	}
	if !v.Probe.Connected || len(v.Current) == 0 {
		semanticFailure(w, model.Fail("invalid_input", "No complete structure evidence to accept"))
		return
	}
	if v.BaselineColumns == nil {
		v.BaselineColumns = map[string][]model.Column{}
	}
	for key, value := range v.Current {
		v.Baseline[key] = value
		v.BaselineColumns[key] = v.CurrentColumns[key]
	}
	for i := range v.Structure {
		if v.Structure[i].Status == "changed" {
			v.Structure[i].Status = "unchanged"
			v.Structure[i].Changes = nil
		}
	}
	if err = s.Store.SaveSourceHealth(v); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) healthOverview(w http.ResponseWriter, r *http.Request) {
	sources, err := s.Store.Sources()
	if err != nil {
		fail(w, 500, err)
		return
	}
	cfg, err := s.Store.HealthConfig()
	if err != nil {
		fail(w, 500, err)
		return
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if r.URL.Query().Get("offset") != "" && (err != nil || offset < 0) {
		fail(w, 400, model.Fail("invalid_input", "Invalid health page"))
		return
	}
	offset = min(offset, len(sources))
	rows := []map[string]any{}
	for _, src := range sources[offset:min(offset+20, len(sources))] {
		st, err := s.Store.Semantics(src.ID)
		if err != nil {
			fail(w, 500, err)
			return
		}
		status := "not_checked"
		health, err := s.Store.SourceHealth(src.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			fail(w, 500, err)
			return
		}
		if err == nil {
			status = "checked"
			if !health.Probe.Connected {
				status = "connection_failed"
			}
			for _, check := range health.Structure {
				if check.Status != "baseline" && check.Status != "unchanged" && status == "checked" {
					status = "structure_incomplete"
				}
				if check.Status == "changed" {
					status = "structure_changed"
				}
			}
			if cfg.Enabled && time.Since(health.CheckedAt) > time.Duration(cfg.IntervalMinutes+1)*time.Minute {
				status = "overdue"
			}
			if health.ConnectionRevision != src.ConnectionRevision || health.PublishedVersion != st.PublishedVersion {
				status = "stale"
			}
		}
		templates := []map[string]any{}
		for _, en := range st.Published.Entries {
			if en.Template != nil && en.Template.Enabled {
				v := s.Engine.PublishedTemplateValidation(src, en)
				if !v.Valid {
					templates = append(templates, map[string]any{"id": en.ID, "name": en.Name, "status": v.Status})
				}
			}
		}
		if !src.Enabled {
			status = "disabled"
		}
		rows = append(rows, map[string]any{"id": src.ID, "name": src.Name, "status": status, "health": health, "invalid_templates": templates})
	}
	agents, err := s.Store.Agents()
	if err != nil {
		fail(w, 500, err)
		return
	}
	expiring := []model.Agent{}
	for _, agent := range agents {
		if agent.Enabled && agent.RevokedAt == nil && agent.ExpiresAt.Before(time.Now().Add(7*24*time.Hour)) && len(expiring) < 100 {
			expiring = append(expiring, agent)
		}
	}
	exporter, err := s.AuditExport.View(r.Context())
	if err != nil {
		fail(w, 500, err)
		return
	}
	var pending int
	err = s.Store.DB.QueryRowContext(r.Context(), `SELECT count(*) FROM audit a WHERE event_kind='management' AND error_code='operation_pending' AND at<$1 AND NOT EXISTS(SELECT 1 FROM audit outcome WHERE outcome.request_id=a.request_id AND outcome.id>a.id)`, time.Now().Add(-3*time.Minute).UnixMilli()).Scan(&pending)
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, map[string]any{"sources": rows, "total": len(sources), "expiring_agents": expiring, "audit_export": exporter.Status, "pending_changes": pending})
}
