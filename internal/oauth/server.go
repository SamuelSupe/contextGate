package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Provider       fosite.OAuth2Provider
	storage        *storage
	Store          *store.Store
	Engine         *engine.Engine
	Base, Resource string
	tokenMu        sync.Mutex
	clientMu       sync.Mutex
}

func New(st *store.Store, en *engine.Engine, base string) *Server {
	s := &Server{Store: st, Engine: en, Base: strings.TrimRight(base, "/"), Resource: strings.TrimRight(base, "/") + "/mcp"}
	s.storage = &storage{st, s.Resource}
	cfg := &fosite.Config{GlobalSecret: st.Vault.Key, AccessTokenLifespan: 15 * time.Minute, RefreshTokenLifespan: 30 * 24 * time.Hour, AuthorizeCodeLifespan: 5 * time.Minute, EnforcePKCE: true, EnablePKCEPlainChallengeMethod: false, RefreshTokenScopes: []string{"offline_access"}, ScopeStrategy: fosite.ExactScopeStrategy}
	s.Provider = compose.Compose(cfg, s.storage, &compose.CommonStrategy{CoreStrategy: compose.NewOAuth2HMACStrategy(cfg)}, compose.OAuth2AuthorizeExplicitFactory, compose.OAuth2RefreshTokenGrantFactory, compose.OAuth2PKCEFactory, compose.OAuth2TokenIntrospectionFactory, compose.OAuth2TokenRevocationFactory)
	return s
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func (s *Server) ResourceMetadata(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"resource": s.Resource, "authorization_servers": []string{s.Base}, "scopes_supported": []string{"db:read", "offline_access"}, "bearer_methods_supported": []string{"header"}})
}
func (s *Server) Metadata(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"issuer": s.Base, "authorization_endpoint": s.Base + "/oauth/authorize", "token_endpoint": s.Base + "/oauth/token", "registration_endpoint": s.Base + "/oauth/register", "revocation_endpoint": s.Base + "/oauth/revoke", "response_types_supported": []string{"code"}, "response_modes_supported": []string{"query"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic", "client_secret_post"}, "scopes_supported": []string{"db:read", "offline_access"}, "client_id_metadata_document_supported": true, "authorization_response_iss_parameter_supported": true})
}
func (s *Server) Registration(w http.ResponseWriter, r *http.Request) {
	var in Registration
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if e := d.Decode(&in); e != nil {
		write(w, 400, map[string]any{"error": "invalid_client_metadata"})
		return
	}
	out, e := s.Register(r.Context(), in, false)
	if e != nil {
		write(w, 400, map[string]any{"error": "invalid_client_metadata", "error_description": e.Error()})
		return
	}
	write(w, 201, out)
}
func (s *Server) authorized(r *http.Request) bool {
	cookie, e := r.Cookie("hub_session")
	if e != nil {
		return false
	}
	_, e = s.Store.CheckSession(cookie.Value)
	return e == nil
}
func (s *Server) prepare(r *http.Request) (fosite.AuthorizeRequester, error) {
	if e := r.ParseForm(); e != nil {
		return nil, e
	}
	if len(r.Form["resource"]) != 1 || r.Form.Get("resource") != s.Resource {
		return nil, fosite.ErrInvalidRequest.WithHint("resource must match the MCP endpoint")
	}
	if r.Form.Get("scope") == "" {
		r.Form.Set("scope", "db:read")
	}
	ar, e := s.Provider.NewAuthorizeRequest(r.Context(), r)
	if e != nil {
		return ar, e
	}
	if !slices.Contains(ar.GetRequestedScopes(), "db:read") {
		return ar, fosite.ErrInvalidScope
	}
	ar.SetRequestedAudience(fosite.Arguments{s.Resource})
	return ar, nil
}
func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("client_id")
	if strings.HasPrefix(id, "https://") {
		reg, e := fetchMetadata(r.Context(), id)
		if e != nil {
			write(w, 400, map[string]any{"error": "invalid_client_metadata"})
			return
		}
		if _, e = s.Register(r.Context(), reg, true); e != nil {
			write(w, 400, map[string]any{"error": "invalid_client_metadata"})
			return
		}
	}
	ar, e := s.prepare(r)
	if e != nil {
		s.Provider.WriteAuthorizeError(r.Context(), w, ar, e)
		return
	}
	if !s.authorized(r) {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return
	}
	requestID := secure.Random(24)
	if e = s.storage.put(r.Context(), "consent", requestID, r.Form, time.Now().Add(5*time.Minute), ""); e != nil {
		write(w, 500, map[string]any{"error": "storage_unavailable"})
		return
	}
	http.Redirect(w, r, "/oauth/consent?request="+url.QueryEscape(requestID), http.StatusSeeOther)
}
func (s *Server) ConsentInfo(w http.ResponseWriter, r *http.Request) {
	var form url.Values
	_, e := s.storage.get(r.Context(), "consent", r.URL.Query().Get("request"), &form)
	if e != nil {
		write(w, 404, map[string]any{"error": "consent_expired"})
		return
	}
	var client Client
	if _, e = s.storage.get(r.Context(), "client", form.Get("client_id"), &client); e != nil {
		write(w, 404, map[string]any{"error": "client_not_found"})
		return
	}
	sources, e := s.Store.Sources()
	if e != nil {
		write(w, 500, map[string]any{"error": "storage_unavailable"})
		return
	}
	visible := []map[string]any{}
	for _, src := range sources {
		if src.Enabled {
			visible = append(visible, map[string]any{"id": src.ID, "name": src.Name, "kind": src.Kind})
		}
	}
	write(w, 200, map[string]any{"client_name": client.Name, "client_id": client.ID, "redirect_uri": form.Get("redirect_uri"), "scope": form.Get("scope"), "sources": visible})
}
func (s *Server) Consent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Request string   `json:"request"`
		Allow   bool     `json:"allow"`
		Sources []string `json:"sources"`
	}
	if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
		write(w, 400, map[string]any{"error": "invalid_request"})
		return
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	var form url.Values
	_, e := s.storage.get(r.Context(), "consent", in.Request, &form)
	if e != nil {
		write(w, 404, map[string]any{"error": "consent_expired"})
		return
	}
	req := r.Clone(r.Context())
	req.Method = http.MethodGet
	// Authorization is reconstructed from the stored form, never the submitted
	// consent body, which Fosite would otherwise try to parse again under the lock.
	req.Body = http.NoBody
	req.ContentLength = 0
	req.Header.Del("Content-Type")
	u := *r.URL
	u.Path = "/oauth/authorize"
	u.RawQuery = form.Encode()
	req.URL = &u
	req.Form = form
	req.PostForm = nil
	ar, e := s.prepare(req)
	if e != nil {
		write(w, 400, map[string]any{"error": "invalid_request"})
		return
	}
	if !in.Allow {
		if e = s.storage.del(r.Context(), "consent", in.Request); e != nil {
			write(w, 500, map[string]any{"error": "storage_unavailable"})
			return
		}
		redirect := ar.GetRedirectURI()
		q := redirect.Query()
		q.Set("error", "access_denied")
		q.Set("state", ar.GetState())
		q.Set("iss", s.Base)
		redirect.RawQuery = q.Encode()
		write(w, 200, map[string]any{"redirect": redirect.String()})
		return
	}
	if len(in.Sources) == 0 {
		write(w, 400, map[string]any{"error": "select_at_least_one_source"})
		return
	}
	for _, id := range in.Sources {
		src, e := s.Store.Source(id)
		if e != nil || !src.Enabled {
			write(w, 400, map[string]any{"error": "invalid_source"})
			return
		}
	}
	var client Client
	if _, e = s.storage.get(r.Context(), "client", form.Get("client_id"), &client); e != nil {
		write(w, 400, map[string]any{"error": "invalid_client"})
		return
	}
	if e = s.storage.del(r.Context(), "consent", in.Request); e != nil {
		write(w, 500, map[string]any{"error": "storage_unavailable"})
		return
	}
	agent := model.Agent{ID: "agent_" + secure.Random(18), Name: client.Name, Enabled: true, Sources: in.Sources, ExpiresAt: time.Now().Add(30 * 24 * time.Hour), CreatedAt: time.Now(), AuthType: "oauth", Revision: 1, ClientID: client.ID}
	if e = s.Store.SaveAgent(agent, ""); e != nil {
		write(w, 500, map[string]any{"error": "storage_unavailable"})
		return
	}
	for _, scope := range ar.GetRequestedScopes() {
		ar.GrantScope(scope)
	}
	ar.GrantAudience(s.Resource)
	session := &fosite.DefaultSession{Subject: agent.ID, Username: "administrator"}
	response, e := s.Provider.NewAuthorizeResponse(r.Context(), ar, session)
	if e != nil {
		write(w, 500, map[string]any{"error": "authorization_failed"})
		return
	}
	response.GetParameters().Set("iss", s.Base)
	rec := &redirectRecorder{header: http.Header{}}
	s.Provider.WriteAuthorizeResponse(r.Context(), rec, ar, response)
	if rec.header.Get("Location") == "" {
		write(w, 500, map[string]any{"error": "authorization_failed"})
		return
	}
	write(w, 200, map[string]any{"redirect": rec.header.Get("Location")})
}

type redirectRecorder struct{ header http.Header }

func (r *redirectRecorder) Header() http.Header         { return r.header }
func (r *redirectRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *redirectRecorder) WriteHeader(int)             {}

func parseOAuthForm(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	// Fosite reparses multipart forms too; finish network reads before tokenMu.
	if err := r.ParseMultipartForm(1 << 20); err != http.ErrNotMultipart {
		return err
	}
	return nil
}

func (s *Server) Token(w http.ResponseWriter, r *http.Request) {
	if e := parseOAuthForm(r); e != nil || r.PostForm.Get("resource") != s.Resource {
		write(w, 400, map[string]any{"error": "invalid_target"})
		return
	}
	// Network reads must finish before serializing token mutations; an
	// unauthenticated slow upload must not delay other clients or revocations.
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	id := r.PostForm.Get("client_id")
	basicID, _, basic := r.BasicAuth()
	if basic {
		id = basicID
	}
	var client Client
	if _, e := s.storage.get(r.Context(), "client", id, &client); e != nil || client.Disabled {
		write(w, 401, map[string]any{"error": "invalid_client"})
		return
	}
	if (client.AuthMethod == "client_secret_basic" && !basic) || (client.AuthMethod == "client_secret_post" && basic) || (client.AuthMethod == "none" && (basic || r.PostForm.Get("client_secret") != "")) {
		write(w, 401, map[string]any{"error": "invalid_client"})
		return
	}
	ar, e := s.Provider.NewAccessRequest(r.Context(), r, &fosite.DefaultSession{})
	if e != nil {
		s.Provider.WriteAccessError(r.Context(), w, ar, e)
		return
	}
	a, e := s.Store.Agent(ar.GetSession().GetSubject())
	if e != nil || !a.Enabled || !a.ExpiresAt.After(time.Now()) || !slices.Contains(ar.GetRequestedAudience(), s.Resource) {
		s.Provider.WriteAccessError(r.Context(), w, ar, fosite.ErrInvalidGrant)
		return
	}
	response, e := s.Provider.NewAccessResponse(r.Context(), ar)
	if e != nil {
		s.Provider.WriteAccessError(r.Context(), w, ar, e)
		return
	}
	s.Provider.WriteAccessResponse(r.Context(), w, ar, response)
}
func (s *Server) Principal(ctx context.Context, token string) (model.Principal, error) {
	_, ar, e := s.Provider.IntrospectToken(ctx, token, fosite.AccessToken, &fosite.DefaultSession{}, "db:read")
	if e != nil {
		return model.Principal{}, e
	}
	if !slices.Contains(ar.GetGrantedAudience(), s.Resource) {
		return model.Principal{}, errors.New("wrong token audience")
	}
	if _, e := s.storage.GetClient(ctx, ar.GetClient().GetID()); e != nil {
		return model.Principal{}, e
	}
	a, e := s.Store.Agent(ar.GetSession().GetSubject())
	if e != nil || !a.Enabled || !a.ExpiresAt.After(time.Now()) {
		return model.Principal{}, errors.New("authorization revoked or expired")
	}
	return model.Principal{AgentID: a.ID}, nil
}
func (s *Server) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := parseOAuthForm(r); err != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	token := r.PostForm.Get("token")
	p, _ := s.Principal(r.Context(), token)
	if p.AgentID == "" {
		_, ar, err := s.Provider.IntrospectToken(r.Context(), token, fosite.RefreshToken, &fosite.DefaultSession{})
		if err == nil {
			p.AgentID = ar.GetSession().GetSubject()
		}
	}
	e := s.Provider.NewRevocationRequest(r.Context(), r)
	s.Provider.WriteRevocationResponse(r.Context(), w, e)
	if e == nil && p.AgentID != "" {
		s.Engine.InvalidateAgent(p.AgentID)
	}
}
