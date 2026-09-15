package server

import (
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"net/http"
	"strings"
	"time"
)

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.Store.AdministratorInitialized()
	if err != nil {
		fail(w, 503, err)
		return
	}
	a, csrf, ok := s.adminSession(r)
	var administrator any
	if ok {
		administrator = a
	}
	write(w, 200, map[string]any{"initialized": initialized, "authenticated": ok, "csrf": csrf, "administrator": administrator})
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if decode(r, &in) != nil || len(in.Password) < 12 || len(in.Password) > 256 {
		fail(w, 400, model.Fail("invalid_input", "Password must contain 12-256 bytes"))
		return
	}
	expected, err := s.Store.Get("setup_hash")
	if err != nil || !secure.Equal(expected, secure.Hash(in.Token)) {
		fail(w, 403, model.Fail("invalid_setup_token", "Invalid or consumed setup token"))
		return
	}
	if in.DisplayName == "" {
		in.DisplayName = in.Username
	}
	intent := model.Audit{At: time.Now().UTC(), EventKind: "security", Operation: "administrator.setup", ActorType: "unauthenticated", Channel: "ui", RequestID: secure.Random(16), ErrorCode: "operation_pending"}
	if err = s.Store.Audit(intent); err != nil {
		fail(w, 503, model.Fail("audit_unavailable", "Initialization was not changed because the audit log is unavailable"))
		return
	}
	a, err := s.Store.SetupAdministrator(in.Username, in.DisplayName, secure.Password(in.Password))
	intent.ErrorCode = ""
	if err != nil {
		intent.ErrorCode = model.ErrorCode(err)
	} else {
		intent.ResourceID = a.ID
	}
	if auditErr := s.Store.Audit(intent); auditErr != nil {
		fail(w, 503, model.Fail("audit_unavailable", "Initialization outcome is pending confirmation"))
		return
	}
	if err != nil {
		administratorFailure(w, err)
		return
	}
	s.createAdministratorSession(w, r, a)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || len(in.Password) > 256 || len(in.Username) > 64 {
		fail(w, 400, model.Fail("invalid_input", "Invalid sign-in fields"))
		return
	}
	a, err := s.Store.AdministratorByUsername(in.Username)
	valid := err == nil && a.Active() && secure.CheckPassword(a.PasswordHash, in.Password)
	if !valid {
		// Unknown or disabled usernames have the same public error and hash workload.
		if err != nil || !a.Active() {
			secure.CheckPassword(s.dummyPasswordHash, in.Password)
		}
		if err = s.authenticationAudit(r, model.Administrator{}, "administrator.login", "invalid_credentials"); err != nil {
			fail(w, 503, err)
			return
		}
		fail(w, 401, model.Fail("invalid_credentials", "Incorrect username or password"))
		return
	}
	s.createAdministratorSession(w, r, a)
}
func (s *Server) createAdministratorSession(w http.ResponseWriter, r *http.Request, a model.Administrator) {
	token, csrf := secure.Random(32), secure.Random(24)
	expiry := time.Now().Add(12 * time.Hour)
	audit := model.Principal{AdministratorID: a.ID, AdministratorUsername: a.Username, Channel: "ui"}.AttributeAudit(model.Audit{At: time.Now().UTC(), EventKind: "security", Operation: "administrator.login", ResourceID: a.ID, RequestID: secure.Random(16), ErrorCode: "operation_pending"})
	if err := s.Store.Audit(audit); err != nil {
		fail(w, 503, model.Fail("audit_unavailable", "Sign-in is unavailable because the audit log cannot be written"))
		return
	}
	err := s.Store.CreateAdministratorSession(a.ID, token, csrf, expiry, a.PasswordHash)
	audit.ErrorCode = ""
	if err != nil {
		audit.ErrorCode = model.ErrorCode(err)
	}
	audit.ElapsedMS = time.Since(audit.At).Milliseconds()
	if auditErr := s.Store.Audit(audit); auditErr != nil {
		s.Store.DeleteSession(token)
		fail(w, 503, model.Fail("audit_unavailable", "Sign-in outcome could not be recorded"))
		return
	}
	if err != nil {
		administratorFailure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "hub_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, Expires: expiry, MaxAge: 12 * 60 * 60})
	write(w, 200, map[string]any{"authenticated": true, "csrf": csrf, "administrator": a})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("hub_session")
	if cookie != nil {
		if err := s.Store.DeleteSession(cookie.Value); err != nil {
			fail(w, 500, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "hub_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	write(w, 200, map[string]any{"ok": true})
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || len(in.Password) < 12 || len(in.Password) > 256 || len(in.Current) > 256 {
		fail(w, 400, model.Fail("invalid_input", "New password must contain 12-256 bytes"))
		return
	}
	p := model.AdministratorPrincipal(r.Context())
	a, err := s.Store.Administrator(p.AdministratorID)
	if err != nil || !secure.CheckPassword(a.PasswordHash, in.Current) {
		fail(w, 403, model.Fail("invalid_credentials", "Incorrect current password"))
		return
	}
	if secure.CheckPassword(a.PasswordHash, in.Password) {
		fail(w, 400, model.Fail("invalid_input", "Choose a different password"))
		return
	}
	a, err = s.Store.ChangeAdministratorPassword(r.Context(), a.ID, a.PasswordHash, secure.Password(in.Password), p.SessionIdentity)
	if err != nil {
		administratorFailure(w, err)
		return
	}
	s.cancelAdministrator(a.ID, p.SessionIdentity, false)
	_, csrf, _ := s.adminSession(r)
	write(w, 200, map[string]any{"authenticated": true, "csrf": csrf, "administrator": a})
}
func (s *Server) EnsureSetup() (string, error) {
	initialized, err := s.Store.AdministratorInitialized()
	if err != nil {
		return "", err
	}
	if initialized {
		return "", nil
	}
	token := secure.Random(24)
	return token, s.Store.Set("setup_hash", secure.Hash(token))
}
