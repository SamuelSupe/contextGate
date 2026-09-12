package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/mcpdbhub/internal/adapter"
	"github.com/SamuelSupe/mcpdbhub/internal/auditexport"
	"github.com/SamuelSupe/mcpdbhub/internal/engine"
	"github.com/SamuelSupe/mcpdbhub/internal/mcpserver"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/oauth"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"github.com/SamuelSupe/mcpdbhub/internal/ui"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Store               *store.Store
	AuditExport         *auditexport.Manager
	Engine              *engine.Engine
	OAuth               *oauth.Server
	PublicURL, FileRoot string
	rateMu              sync.Mutex
	authRate            map[string]*rate.Limiter
}

func New(st *store.Store, publicURL, fileRoot string) (*Server, error) {
	u, e := url.Parse(publicURL)
	if e != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || u.Path != "" && u.Path != "/" {
		return nil, errors.New("public URL must be an HTTP(S) origin")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || !(u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())) {
		return nil, errors.New("remote public URL requires HTTPS")
	}
	en := engine.New(st)
	en.FileRoot = fileRoot
	s := &Server{Store: st, Engine: en, PublicURL: strings.TrimRight(publicURL, "/"), FileRoot: fileRoot, authRate: map[string]*rate.Limiter{}}
	s.OAuth = oauth.New(st, en, s.PublicURL)
	s.AuditExport, e = auditexport.New(st)
	if e != nil {
		en.Close()
		return nil, e
	}
	return s, nil
}
func (s *Server) Close() {
	s.Engine.Close()
	s.AuditExport.Close()
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	write(w, status, map[string]any{"error": engine.PublicError(err)})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func (s *Server) admin(r *http.Request) (string, bool) {
	cookie, e := r.Cookie("hub_session")
	if e != nil {
		return "", false
	}
	csrf, e := s.Store.CheckSession(cookie.Value)
	return csrf, e == nil
}
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrf, ok := s.admin(r)
		if !ok {
			fail(w, 401, model.Fail("unauthorized", "administrator login required"))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !secure.Equal(csrf, r.Header.Get("X-CSRF-Token")) {
			fail(w, 403, model.Fail("csrf_failed", "invalid CSRF token"))
			return
		}
		next(w, r)
	}
}
func (s *Server) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		s.rateMu.Lock()
		lim := s.authRate[ip]
		if lim == nil {
			if len(s.authRate) > 10000 {
				s.rateMu.Unlock()
				write(w, 429, map[string]any{"error": "rate_limited"})
				return
			}
			lim = rate.NewLimiter(rate.Every(6*time.Second), 10)
			s.authRate[ip] = lim
		}
		ok := lim.Allow()
		s.rateMu.Unlock()
		if !ok {
			write(w, 429, map[string]any{"error": "rate_limited"})
			return
		}
		next(w, r)
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.semanticRoutes(mux)
	s.ontologyRoutes(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if e := s.Store.DB.PingContext(ctx); e != nil {
			write(w, 503, map[string]any{"status": "unavailable"})
			return
		}
		write(w, 200, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/setup", s.limited(s.setup))
	mux.HandleFunc("POST /api/login", s.limited(s.login))
	mux.HandleFunc("POST /api/logout", s.requireAdmin(s.logout))
	mux.HandleFunc("GET /api/sources", s.requireAdmin(s.sources))
	mux.HandleFunc("POST /api/sources", s.requireAdmin(s.saveSource))
	mux.HandleFunc("PUT /api/sources/{id}", s.requireAdmin(s.saveSource))
	mux.HandleFunc("DELETE /api/sources/{id}", s.requireAdmin(s.deleteSource))
	mux.HandleFunc("POST /api/sources/test", s.requireAdmin(s.testSource))
	mux.HandleFunc("POST /api/sources/{id}/test", s.requireAdmin(s.testSource))
	mux.HandleFunc("POST /api/query", s.requireAdmin(s.query))
	mux.HandleFunc("GET /api/sources/{id}/objects", s.requireAdmin(s.discover))
	mux.HandleFunc("GET /api/agents", s.requireAdmin(s.agents))
	mux.HandleFunc("POST /api/agents", s.requireAdmin(s.saveAgent))
	mux.HandleFunc("PUT /api/agents/{id}", s.requireAdmin(s.saveAgent))
	mux.HandleFunc("DELETE /api/agents/{id}", s.requireAdmin(s.revokeAgent))
	mux.HandleFunc("POST /api/agents/{id}/token", s.requireAdmin(s.rotateAgent))
	mux.HandleFunc("GET /api/audit", s.requireAdmin(s.audit))
	mux.HandleFunc("GET /api/catalog", s.requireAdmin(func(w http.ResponseWriter, r *http.Request) { write(w, 200, adapter.Catalog()) }))
	mux.HandleFunc("GET /api/settings", s.requireAdmin(s.settings))
	mux.HandleFunc("GET /api/settings/audit-export", s.requireAdmin(s.auditExportSettings))
	mux.HandleFunc("PUT /api/settings/audit-export", s.requireAdmin(s.saveAuditExport))
	mux.HandleFunc("POST /api/settings/audit-export/test", s.requireAdmin(s.testAuditExport))
	mux.HandleFunc("POST /api/password", s.requireAdmin(s.password))
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.OAuth.ResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.OAuth.ResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.OAuth.Metadata)
	mux.HandleFunc("GET /oauth/authorize", s.limited(s.OAuth.Authorize))
	mux.HandleFunc("POST /oauth/register", s.limited(s.OAuth.Registration))
	mux.HandleFunc("POST /oauth/token", s.OAuth.Token)
	mux.HandleFunc("POST /oauth/revoke", s.OAuth.Revoke)
	mux.HandleFunc("GET /api/oauth/consent", s.requireAdmin(s.OAuth.ConsentInfo))
	mux.HandleFunc("POST /api/oauth/consent", s.requireAdmin(s.OAuth.Consent))
	mux.HandleFunc("POST /api/oauth/clients", s.requireAdmin(s.OAuth.Registration))
	mux.HandleFunc("GET /api/oauth/clients", s.requireAdmin(s.OAuth.ListClients))
	mux.HandleFunc("PUT /api/oauth/clients/{id}", s.requireAdmin(s.OAuth.UpdateClient))
	mux.HandleFunc("POST /api/oauth/clients/{id}/secret", s.requireAdmin(s.OAuth.RotateClientSecret))
	mux.HandleFunc("DELETE /api/oauth/clients/{id}", s.requireAdmin(s.OAuth.DeleteClient))
	stream := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		p, _ := r.Context().Value(principalKey{}).(model.Principal)
		return mcpserver.Server(s.Engine, p)
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		scheme, token, ok := strings.Cut(auth, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			s.challenge(w)
			return
		}
		p, e := s.principal(r.Context(), token)
		if e != nil {
			s.challenge(w)
			return
		}
		stream.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, 404, model.Fail("not_found", "API route not found"))
	})
	mux.Handle("/", ui.Handler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		u, _ := url.Parse(s.PublicURL)
		host := r.Host
		valid := host == u.Host
		h, p, _ := net.SplitHostPort(host)
		if (h == "localhost" || h == "127.0.0.1" || h == "::1") && p == u.Port() {
			valid = true
		}
		if !valid {
			http.Error(w, "invalid Host", 403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.PublicURL && origin != u.Scheme+"://"+host {
			http.Error(w, "invalid Origin", 403)
			return
		}
		if r.URL.Query().Has("access_token") {
			http.Error(w, "tokens must use Authorization header", 400)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type principalKey struct{}

func (s *Server) principal(ctx context.Context, token string) (model.Principal, error) {
	if strings.HasPrefix(token, "hub_") {
		a, e := s.Store.TokenAgent(token)
		if e != nil || !a.Enabled || a.RevokedAt != nil || !a.ExpiresAt.After(time.Now()) {
			return model.Principal{}, errors.New("invalid token")
		}
		return model.Principal{AgentID: a.ID, CredentialValid: func() bool {
			fresh, err := s.Store.TokenAgent(token)
			return err == nil && fresh.Enabled && fresh.RevokedAt == nil && fresh.ExpiresAt.After(time.Now())
		}}, nil
	}
	p, err := s.OAuth.Principal(ctx, token)
	if err == nil {
		p.CredentialValid = func() bool { _, err := s.OAuth.Principal(ctx, token); return err == nil }
	}
	return p, err
}
func (s *Server) challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+s.PublicURL+`/.well-known/oauth-protected-resource", scope="db:read"`)
	write(w, 401, map[string]any{"error": "unauthorized"})
}
