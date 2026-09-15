package server

import (
	"context"
	"database/sql"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type administratorRequest struct {
	administrator string
	session       string
	cancel        context.CancelFunc
}

func (s *Server) administratorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/administrators", s.requireAdmin(s.administrators))
	mux.HandleFunc("POST /api/administrators", s.requireAdmin(s.createAdministrator))
	mux.HandleFunc("PUT /api/administrators/{id}", s.requireAdmin(s.updateAdministrator))
	mux.HandleFunc("POST /api/administrators/{id}/reset-password", s.requireAdmin(s.resetAdministratorPassword))
	mux.HandleFunc("GET /api/audit/administrators", s.requireAdmin(s.auditAdministrators))
}

func superAdministratorRoute(r *http.Request) bool {
	path := r.URL.Path
	return strings.HasPrefix(path, "/api/administrators") || strings.HasPrefix(path, "/api/settings/audit-export") || path == "/api/settings/diagnostics" || path == "/api/settings/health"
}

func (s *Server) administratorContext(r *http.Request, a model.Administrator) (*http.Request, func()) {
	cookie, _ := r.Cookie("hub_session")
	session := secure.Hash(cookie.Value)
	ctx, cancel := context.WithCancel(r.Context())
	key := secure.Random(16)
	s.administratorMu.Lock()
	if s.administratorRequests == nil {
		s.administratorRequests = map[string]administratorRequest{}
	}
	s.administratorRequests[key] = administratorRequest{a.ID, session, cancel}
	s.administratorMu.Unlock()
	p := model.Principal{Admin: true, Preview: true, AdministratorID: a.ID, AdministratorUsername: a.Username, AdministratorRole: a.Role, CredentialVersion: a.SecurityVersion, SessionIdentity: session, Channel: "ui", CredentialValid: func() bool {
		fresh, _, err := s.Store.AdministratorSession(cookie.Value)
		return err == nil && fresh.SecurityVersion == a.SecurityVersion
	}}
	return r.WithContext(model.WithConfigurationPrincipal(ctx, p)), func() {
		cancel()
		s.administratorMu.Lock()
		delete(s.administratorRequests, key)
		s.administratorMu.Unlock()
	}
}

// Cancel only this person's work. Query Agent credentials remain independent,
// including Agents used in previews by another administrator.
func (s *Server) cancelAdministrator(id, keepSession string, configuration bool) {
	s.administratorMu.Lock()
	for _, job := range s.administratorRequests {
		if job.administrator == id && job.session != keepSession {
			job.cancel()
		}
	}
	s.administratorMu.Unlock()
	if configuration {
		s.configurationMu.Lock()
		for _, job := range s.configurationJobs {
			if job.administrator == id {
				job.cancel()
			}
		}
		s.configurationMu.Unlock()
	}
}

func (s *Server) administrators(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.Administrators()
	if err != nil {
		fail(w, 500, err)
		return
	}
	identities, err := s.Store.ConfigurationAgents()
	if err != nil {
		fail(w, 500, err)
		return
	}
	type row struct {
		model.Administrator
		Configuration *model.ConfigurationAgent `json:"configuration,omitempty"`
	}
	out := []row{}
	for _, a := range all {
		v := row{Administrator: a}
		for _, identity := range identities {
			if identity.AdministratorID == a.ID {
				copy := identity
				v.Configuration = &copy
				break
			}
		}
		out = append(out, v)
	}
	write(w, 200, map[string]any{"administrators": out})
}
func (s *Server) auditAdministrators(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.Administrators()
	if err != nil {
		fail(w, 500, err)
		return
	}
	out := []map[string]any{}
	for _, a := range all {
		out = append(out, map[string]any{"id": a.ID, "username": a.Username, "display_name": a.DisplayName})
	}
	write(w, 200, map[string]any{"administrators": out})
}
func (s *Server) createAdministrator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid administrator fields"))
		return
	}
	password := secure.Random(24)
	a, err := s.Store.CreateAdministrator(r.Context(), in.Username, in.DisplayName, in.Role, secure.Password(password))
	if err != nil {
		administratorFailure(w, err)
		return
	}
	write(w, 200, map[string]any{"id": a.ID, "administrator": a, "temporary_password": password})
}
func (s *Server) updateAdministrator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Enabled     bool   `json:"enabled"`
		Revision    int64  `json:"revision,string"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, model.Fail("invalid_input", "Invalid administrator fields"))
		return
	}
	a, revoke, err := s.Store.UpdateAdministrator(r.Context(), r.PathValue("id"), in.DisplayName, in.Role, in.Enabled, in.Revision)
	if err != nil {
		administratorFailure(w, err)
		return
	}
	if revoke {
		s.cancelAdministrator(a.ID, "", true)
	}
	write(w, 200, a)
}
func (s *Server) resetAdministratorPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64 `json:"revision,string"`
	}
	if decode(r, &in) != nil || in.Revision < 1 {
		fail(w, 400, model.Fail("invalid_input", "Current revision is required"))
		return
	}
	if r.PathValue("id") == model.AdministratorPrincipal(r.Context()).AdministratorID {
		fail(w, 403, model.Fail("forbidden", "Use your personal password settings"))
		return
	}
	password := secure.Random(24)
	a, err := s.Store.ResetAdministratorPassword(r.Context(), r.PathValue("id"), secure.Password(password), in.Revision)
	if err != nil {
		administratorFailure(w, err)
		return
	}
	s.cancelAdministrator(a.ID, "", true)
	write(w, 200, map[string]any{"id": a.ID, "revision": strconv.FormatInt(a.Revision, 10), "administrator": a, "temporary_password": password})
}

func (s *Server) authenticationAudit(r *http.Request, a model.Administrator, operation, code string) error {
	p := model.Principal{AdministratorID: a.ID, AdministratorUsername: a.Username, Channel: "ui"}
	audit := p.AttributeAudit(model.Audit{At: time.Now().UTC(), EventKind: "security", Operation: operation, ErrorCode: code, RequestID: secure.Random(16), ResourceID: a.ID})
	if a.ID == "" {
		audit.ActorType = "unauthenticated"
	}
	return s.Store.Audit(audit)
}

func administratorFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch model.ErrorCode(err) {
	case "invalid_input", "already_initialized":
		status = http.StatusBadRequest
	case "conflict":
		status = http.StatusConflict
	case "not_found":
		status = http.StatusNotFound
	case "forbidden":
		status = http.StatusForbidden
	case "unauthorized":
		status = http.StatusUnauthorized
	}
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
		err = model.Fail("not_found", "Administrator or configuration identity not found")
	}
	fail(w, status, err)
}
