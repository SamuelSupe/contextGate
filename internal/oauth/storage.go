package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/ory/fosite"
	"net/url"
	"time"
)

type Client struct {
	fosite.DefaultClient
	Name       string    `json:"client_name"`
	AuthMethod string    `json:"token_endpoint_auth_method"`
	Disabled   bool      `json:"disabled"`
	Revision   int64     `json:"revision,string"`
	CreatedAt  time.Time `json:"created_at"`
}
type storage struct {
	store    *store.Store
	resource string
}

func (s *storage) put(ctx context.Context, kind, id string, v any, exp time.Time, requestID string) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	expires := int64(0)
	if !exp.IsZero() {
		expires = exp.Unix()
	}
	_, e = s.store.DB.ExecContext(ctx, "INSERT INTO oauth(kind,id,value,expires,request_id,active) VALUES($1,$2,$3,$4,$5,1) ON CONFLICT(kind,id) DO UPDATE SET value=excluded.value,expires=excluded.expires,request_id=excluded.request_id,active=1", kind, id, s.store.Vault.Seal(b, "oauth:"+kind+":"+id), expires, requestID)
	return e
}
func (s *storage) get(ctx context.Context, kind, id string, v any) (bool, error) {
	var ciphertext string
	var active int
	e := s.store.DB.QueryRowContext(ctx, "SELECT value,active FROM oauth WHERE kind=$1 AND id=$2 AND (expires=0 OR expires>$3)", kind, id, time.Now().Unix()).Scan(&ciphertext, &active)
	if errors.Is(e, sql.ErrNoRows) {
		return false, fosite.ErrNotFound
	}
	if e != nil {
		return false, e
	}
	b, e := s.store.Vault.Open(ciphertext, "oauth:"+kind+":"+id)
	if e != nil {
		return false, e
	}
	return active == 1, json.Unmarshal(b, v)
}
func (s *storage) del(ctx context.Context, kind, id string) error {
	_, e := s.store.DB.ExecContext(ctx, "DELETE FROM oauth WHERE kind=$1 AND id=$2", kind, id)
	return e
}
func (s *storage) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	var c Client
	_, e := s.get(ctx, "client", id, &c)
	if e != nil {
		return nil, e
	}
	if c.Disabled {
		return nil, fosite.ErrNotFound
	}
	return &c.DefaultClient, nil
}
func (s *storage) ClientAssertionJWTValid(ctx context.Context, jti string) error {
	var v string
	_, e := s.get(ctx, "jti", jti, &v)
	if errors.Is(e, fosite.ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	return fosite.ErrJTIKnown
}
func (s *storage) SetClientAssertionJWT(ctx context.Context, jti string, exp time.Time) error {
	return s.put(ctx, "jti", jti, "used", exp, "")
}
func (s *storage) saveRequest(ctx context.Context, kind, id string, r fosite.Requester) error {
	form := url.Values{}
	for _, k := range []string{"client_id", "redirect_uri", "scope", "resource", "audience", "code_challenge", "code_challenge_method"} {
		if values := r.GetRequestForm()[k]; len(values) > 0 {
			form[k] = append([]string{}, values...)
		}
	}
	req := &fosite.Request{ID: r.GetID(), RequestedAt: r.GetRequestedAt(), Client: r.GetClient(), RequestedScope: r.GetRequestedScopes(), GrantedScope: r.GetGrantedScopes(), Form: form, Session: r.GetSession().Clone(), RequestedAudience: r.GetRequestedAudience(), GrantedAudience: r.GetGrantedAudience()}
	expiry := time.Now().Add(30 * 24 * time.Hour)
	if kind == "access" {
		expiry = r.GetSession().GetExpiresAt(fosite.AccessToken)
	}
	if kind == "code" || kind == "pkce" {
		expiry = time.Now().Add(10 * time.Minute)
	}
	return s.put(ctx, kind, id, req, expiry, r.GetID())
}
func (s *storage) loadRequest(ctx context.Context, kind, id string) (fosite.Requester, bool, error) {
	r := fosite.NewRequest()
	r.Session = &fosite.DefaultSession{}
	active, e := s.get(ctx, kind, id, r)
	if e != nil {
		return nil, false, e
	}
	return r, active, nil
}
func (s *storage) CreateAuthorizeCodeSession(ctx context.Context, id string, r fosite.Requester) error {
	return s.saveRequest(ctx, "code", id, r)
}
func (s *storage) GetAuthorizeCodeSession(ctx context.Context, id string, _ fosite.Session) (fosite.Requester, error) {
	r, active, e := s.loadRequest(ctx, "code", id)
	if e != nil {
		return nil, e
	}
	if !active {
		return r, fosite.ErrInvalidatedAuthorizeCode
	}
	return r, nil
}
func (s *storage) InvalidateAuthorizeCodeSession(ctx context.Context, id string) error {
	r, e := s.store.DB.ExecContext(ctx, "UPDATE oauth SET active=0 WHERE kind='code' AND id=$1 AND active=1", id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fosite.ErrInvalidatedAuthorizeCode
	}
	return nil
}
func (s *storage) CreatePKCERequestSession(ctx context.Context, id string, r fosite.Requester) error {
	return s.saveRequest(ctx, "pkce", id, r)
}
func (s *storage) GetPKCERequestSession(ctx context.Context, id string, _ fosite.Session) (fosite.Requester, error) {
	r, _, e := s.loadRequest(ctx, "pkce", id)
	return r, e
}
func (s *storage) DeletePKCERequestSession(ctx context.Context, id string) error {
	return s.del(ctx, "pkce", id)
}
func (s *storage) CreateAccessTokenSession(ctx context.Context, id string, r fosite.Requester) error {
	return s.saveRequest(ctx, "access", id, r)
}
func (s *storage) GetAccessTokenSession(ctx context.Context, id string, _ fosite.Session) (fosite.Requester, error) {
	r, active, e := s.loadRequest(ctx, "access", id)
	if e != nil {
		return nil, e
	}
	if !active {
		return r, fosite.ErrInactiveToken
	}
	return r, nil
}
func (s *storage) DeleteAccessTokenSession(ctx context.Context, id string) error {
	return s.del(ctx, "access", id)
}
func (s *storage) CreateRefreshTokenSession(ctx context.Context, id, _ string, r fosite.Requester) error {
	return s.saveRequest(ctx, "refresh", id, r)
}
func (s *storage) GetRefreshTokenSession(ctx context.Context, id string, _ fosite.Session) (fosite.Requester, error) {
	r, active, e := s.loadRequest(ctx, "refresh", id)
	if e != nil {
		return nil, e
	}
	if !active {
		return r, fosite.ErrInactiveToken
	}
	return r, nil
}
func (s *storage) DeleteRefreshTokenSession(ctx context.Context, id string) error {
	return s.del(ctx, "refresh", id)
}
func (s *storage) RevokeRefreshToken(ctx context.Context, id string) error {
	_, e := s.store.DB.ExecContext(ctx, "UPDATE oauth SET active=0 WHERE kind='refresh' AND request_id=$1", id)
	return e
}
func (s *storage) RevokeAccessToken(ctx context.Context, id string) error {
	_, e := s.store.DB.ExecContext(ctx, "DELETE FROM oauth WHERE kind='access' AND request_id=$1", id)
	return e
}
func (s *storage) RotateRefreshToken(ctx context.Context, requestID, id string) error {
	result, e := s.store.DB.ExecContext(ctx, "UPDATE oauth SET active=0 WHERE kind='refresh' AND id=$1 AND active=1", id)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fosite.ErrInactiveToken
	}
	return s.RevokeAccessToken(ctx, requestID)
}
