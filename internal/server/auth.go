package server

import (
	"database/sql"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"net/http"
	"strings"
	"time"
)

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	_, e := s.Store.Get("admin_password")
	csrf, ok := s.admin(r)
	write(w, 200, map[string]any{"initialized": e == nil, "authenticated": ok, "csrf": csrf})
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if e := decode(r, &in); e != nil || len(in.Password) < 12 || len(in.Password) > 256 {
		fail(w, 400, model.Fail("invalid_input", "password must contain 12-256 bytes"))
		return
	}
	expected, e := s.Store.Get("setup_hash")
	if e != nil || !secure.Equal(expected, secure.Hash(in.Token)) {
		fail(w, 403, model.Fail("invalid_setup_token", "invalid or consumed setup token"))
		return
	}
	hash := secure.Password(in.Password)
	if e = s.Store.Setup(hash); e != nil {
		fail(w, 409, model.Fail("already_initialized", "administrator is already initialized"))
		return
	}
	s.createSession(w, r, hash)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if e := decode(r, &in); e != nil || len(in.Password) > 256 {
		fail(w, 400, model.Fail("invalid_input", "invalid password"))
		return
	}
	hash, e := s.Store.Get("admin_password")
	if e != nil || !secure.CheckPassword(hash, in.Password) {
		fail(w, 401, model.Fail("invalid_credentials", "incorrect administrator password"))
		return
	}
	s.createSession(w, r, hash)
}
func (s *Server) createSession(w http.ResponseWriter, r *http.Request, passwordHash string) {
	token, csrf := secure.Random(32), secure.Random(24)
	expiry := time.Now().Add(12 * time.Hour)
	if e := s.Store.Session(token, csrf, expiry, passwordHash); e != nil {
		status := http.StatusInternalServerError
		if model.ErrorCode(e) == "unauthorized" {
			status = http.StatusUnauthorized
		}
		fail(w, status, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "hub_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, Expires: expiry, MaxAge: 12 * 60 * 60})
	write(w, 200, map[string]any{"authenticated": true, "csrf": csrf})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("hub_session")
	if cookie != nil {
		s.Store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "hub_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	write(w, 200, map[string]any{"ok": true})
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if e := decode(r, &in); e != nil || len(in.Password) < 12 || len(in.Password) > 256 {
		fail(w, 400, model.Fail("invalid_input", "new password must contain 12-256 bytes"))
		return
	}
	hash, e := s.Store.Get("admin_password")
	if e != nil || !secure.CheckPassword(hash, in.Current) {
		fail(w, 403, model.Fail("invalid_credentials", "incorrect current password"))
		return
	}
	nextHash := secure.Password(in.Password)
	if e = s.Store.ChangeAdminPassword(hash, nextHash); e != nil {
		status := http.StatusInternalServerError
		if model.ErrorCode(e) == "conflict" {
			status = http.StatusConflict
		}
		fail(w, status, e)
		return
	}
	s.createSession(w, r, nextHash)
}
func (s *Server) EnsureSetup() (string, error) {
	if _, e := s.Store.Get("admin_password"); e == nil {
		return "", nil
	} else if !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	token := secure.Random(24)
	return token, s.Store.Set("setup_hash", secure.Hash(token))
}
