package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type configurationJob struct {
	agent  string
	cancel context.CancelFunc
}

func configurationActive(a model.ConfigurationAgent) bool {
	return a.ID != "" && a.RevokedAt == nil && a.ExpiresAt.After(time.Now())
}

func (s *Server) configurationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/configuration-agents", s.requireAdmin(s.configurationAgents))
	mux.HandleFunc("POST /api/configuration-agents", s.requireAdmin(s.createConfigurationAgent))
	mux.HandleFunc("DELETE /api/configuration-agents/{id}", s.requireAdmin(s.revokeConfigurationAgent))
	stream := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		a := r.Context().Value(configurationCallerKey{}).(model.ConfigurationAgent)
		return s.configurationMCP(a)
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	mux.Handle("/mcp/config", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		var a model.ConfigurationAgent
		if ok && strings.EqualFold(scheme, "Bearer") && strings.HasPrefix(token, "cfg_token_") && len(token) < 256 {
			a, _ = s.Store.ConfigurationToken(token)
		}
		if !configurationActive(a) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="ContextGate Configuration MCP"`)
			fail(w, 401, model.Fail("unauthorized", "A dedicated Configuration MCP token is required"))
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		stream.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), configurationCallerKey{}, a)))
	}))
}

type configurationCallerKey struct{}

func (s *Server) configurationAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.Store.ConfigurationAgents()
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, map[string]any{"agents": agents, "endpoint": s.PublicURL + "/mcp/config", "publication": "administrator_only"})
}

func (s *Server) createConfigurationAgent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string    `json:"name"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid configuration credential"))
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.ExpiresAt.IsZero() {
		in.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	if in.Name == "" || len(in.Name) > 120 || !in.ExpiresAt.After(time.Now()) || in.ExpiresAt.After(time.Now().Add(30*24*time.Hour)) {
		fail(w, 400, model.Fail("invalid_input", "A name and an expiry within 30 days are required"))
		return
	}
	a, token, err := s.Store.CreateConfigurationAgent(in.Name, in.ExpiresAt)
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, map[string]any{"id": a.ID, "agent": a, "token": token, "endpoint": s.PublicURL + "/mcp/config"})
}

func (s *Server) revokeConfigurationAgent(w http.ResponseWriter, r *http.Request) {
	s.configurationMu.Lock()
	defer s.configurationMu.Unlock()
	s.Store.Mutations.Lock()
	err := s.Store.RevokeConfigurationAgent(r.PathValue("id"))
	s.Store.Mutations.Unlock()
	if err != nil {
		fail(w, 404, model.Fail("not_found", "Configuration Agent not found"))
		return
	}
	for _, job := range s.configurationJobs {
		if job.agent == r.PathValue("id") {
			job.cancel()
		}
	}
	s.Engine.InvalidateAgent(r.PathValue("id"))
	write(w, 200, map[string]bool{"revoked": true})
}

func (s *Server) startConfigurationCall(ctx context.Context, id string) (context.Context, func(), error) {
	s.configurationMu.Lock()
	defer s.configurationMu.Unlock()
	a, err := s.Store.ConfigurationAgent(id)
	if err != nil || !configurationActive(a) {
		return nil, nil, model.Fail("unauthorized", "Configuration credential revoked or expired")
	}
	count := 0
	for _, job := range s.configurationJobs {
		if job.agent == id {
			count++
		}
	}
	if count >= 2 || len(s.configurationJobs) >= 8 {
		return nil, nil, model.Fail("busy", "Configuration concurrency limit reached; retry after the current call")
	}
	deadline := time.Now().Add(2 * time.Minute)
	if a.ExpiresAt.Before(deadline) {
		deadline = a.ExpiresAt
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	key := secure.Random(16)
	if s.configurationJobs == nil {
		s.configurationJobs = map[string]configurationJob{}
	}
	s.configurationJobs[key] = configurationJob{id, cancel}
	p := model.Principal{Admin: true, Preview: true, AgentID: id, CredentialValid: func() bool {
		fresh, err := s.Store.ConfigurationAgent(id)
		return err == nil && configurationActive(fresh)
	}}
	return model.WithConfigurationPrincipal(ctx, p), func() {
		cancel()
		s.configurationMu.Lock()
		delete(s.configurationJobs, key)
		s.configurationMu.Unlock()
	}, nil
}
