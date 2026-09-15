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
	administrator string
	agent         string
	cancel        context.CancelFunc
}

func configurationActive(a model.ConfigurationAgent) bool {
	return a.ID != "" && a.AdministratorID != "" && a.RevokedAt == nil && a.ExpiresAt.After(time.Now())
}

func (s *Server) configurationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/configuration-agents", s.requireAdmin(s.configurationAgents))
	mux.HandleFunc("POST /api/configuration-agents", s.requireAdmin(s.createConfigurationAgent))
	mux.HandleFunc("POST /api/configuration-agents/{id}/token", s.requireAdmin(s.createConfigurationAgent))
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
		if !s.configurationOwnerActive(a) {
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
	p := model.AdministratorPrincipal(r.Context())
	filtered := []model.ConfigurationAgent{}
	for _, a := range agents {
		if a.AdministratorID == p.AdministratorID {
			filtered = append(filtered, a)
		}
	}
	write(w, 200, map[string]any{"agents": filtered, "endpoint": s.PublicURL + "/mcp/config", "publication": "administrator_only"})
}

func (s *Server) createConfigurationAgent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string    `json:"name"`
		Revision  int64     `json:"revision,string"`
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
	if len(in.Name) > 120 || !in.ExpiresAt.After(time.Now()) || in.ExpiresAt.After(time.Now().Add(30*24*time.Hour)) {
		fail(w, 400, model.Fail("invalid_input", "A name and an expiry within 30 days are required"))
		return
	}
	p := model.AdministratorPrincipal(r.Context())
	identity, err := s.Store.AdministratorConfiguration(p.AdministratorID)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if id := r.PathValue("id"); id != "" && id != identity.ID {
		fail(w, 403, model.Fail("forbidden", "You can issue only your own configuration token"))
		return
	}
	if r.PathValue("id") != "" && in.Revision < 1 {
		fail(w, 400, model.Fail("invalid_input", "Current identity revision is required"))
		return
	}
	if r.PathValue("id") == "" && configurationActive(identity) {
		fail(w, 409, model.Fail("conflict", "A token already exists; rotate your fixed identity instead"))
		return
	}
	if in.Revision == 0 {
		in.Revision = identity.Revision
	}
	a, token, err := s.Store.IssueConfigurationToken(r.Context(), p.AdministratorID, in.Name, in.ExpiresAt, in.Revision)
	if err != nil {
		administratorFailure(w, err)
		return
	}
	s.cancelConfigurationIdentity(a.ID)
	write(w, 200, map[string]any{"id": a.ID, "agent": a, "token": token, "endpoint": s.PublicURL + "/mcp/config"})
}

func (s *Server) configurationOwnerActive(a model.ConfigurationAgent) bool {
	if !configurationActive(a) {
		return false
	}
	owner, err := s.Store.Administrator(a.AdministratorID)
	return err == nil && owner.Active() && !owner.MustChangePassword
}
func (s *Server) cancelConfigurationIdentity(id string) {
	s.configurationMu.Lock()
	for _, job := range s.configurationJobs {
		if job.agent == id {
			job.cancel()
		}
	}
	s.configurationMu.Unlock()
	s.Engine.InvalidateAgent(id)
}
func (s *Server) revokeConfigurationAgent(w http.ResponseWriter, r *http.Request) {
	s.Store.Mutations.Lock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		s.Store.Mutations.Unlock()
		fail(w, 401, err)
		return
	}
	a, err := s.Store.ConfigurationAgent(r.PathValue("id"))
	if err != nil {
		s.Store.Mutations.Unlock()
		fail(w, 404, model.Fail("not_found", "Configuration identity not found"))
		return
	}
	p := model.AdministratorPrincipal(r.Context())
	if a.AdministratorID != p.AdministratorID && p.AdministratorRole != model.RoleSuperAdministrator {
		s.Store.Mutations.Unlock()
		fail(w, 403, model.Fail("forbidden", "You can revoke only your own configuration token"))
		return
	}
	err = s.Store.RevokeConfigurationAgent(a.ID)
	s.Store.Mutations.Unlock()
	if err != nil {
		fail(w, 500, err)
		return
	}
	s.cancelConfigurationIdentity(a.ID)
	write(w, 200, map[string]bool{"revoked": true})
}

func (s *Server) startConfigurationCall(ctx context.Context, identity model.ConfigurationAgent) (context.Context, func(), error) {
	s.configurationMu.Lock()
	defer s.configurationMu.Unlock()
	a, err := s.Store.ConfigurationAgent(identity.ID)
	if err != nil || a.Revision != identity.Revision || !s.configurationOwnerActive(a) {
		return nil, nil, model.Fail("unauthorized", "Configuration credential revoked or expired")
	}
	count := 0
	for _, job := range s.configurationJobs {
		if job.agent == a.ID {
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
	s.configurationJobs[key] = configurationJob{administrator: a.AdministratorID, agent: a.ID, cancel: cancel}
	owner, _ := s.Store.Administrator(a.AdministratorID)
	p := model.Principal{Admin: true, Preview: true, AgentID: a.ID, AdministratorID: owner.ID, AdministratorUsername: owner.Username, AdministratorRole: owner.Role, ConfigurationAgentID: a.ID, CredentialVersion: a.Revision, Channel: "configuration_mcp", CredentialValid: func() bool {
		fresh, err := s.Store.ConfigurationAgent(a.ID)
		return err == nil && fresh.Revision == a.Revision && s.configurationOwnerActive(fresh)
	}}
	return model.WithConfigurationPrincipal(ctx, p), func() {
		cancel()
		s.configurationMu.Lock()
		delete(s.configurationJobs, key)
		s.configurationMu.Unlock()
	}, nil
}
