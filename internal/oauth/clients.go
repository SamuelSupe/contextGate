package oauth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/ory/fosite"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"
)

type Registration struct {
	ClientID      string   `json:"client_id,omitempty"`
	Name          string   `json:"client_name"`
	RedirectURIs  []string `json:"redirect_uris"`
	AuthMethod    string   `json:"token_endpoint_auth_method"`
	GrantTypes    []string `json:"grant_types,omitempty"`
	ResponseTypes []string `json:"response_types,omitempty"`
}

func validateRedirect(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Fragment != "" || u.User != nil {
		return errors.New("invalid redirect URI")
	}
	if u.Scheme == "https" && u.Hostname() != "" {
		return nil
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	if strings.Contains(u.Scheme, ".") && u.Host == "" && u.Path != "" {
		return nil
	}
	return errors.New("redirect URI requires HTTPS, a loopback IP, or a reverse-domain native application scheme")
}
func (s *Server) Register(ctx context.Context, in Registration, cimd bool) (map[string]any, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	var n int
	if e := s.Store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM oauth WHERE kind='client' AND id<>$1", in.ClientID).Scan(&n); e != nil {
		return nil, e
	}
	if n >= 1000 {
		return nil, errors.New("client registration limit reached")
	}
	if len(in.Name) < 1 || len(in.Name) > 120 || len(in.RedirectURIs) < 1 || len(in.RedirectURIs) > 10 {
		return nil, errors.New("client name and 1-10 redirect URIs are required")
	}
	for _, u := range in.RedirectURIs {
		if e := validateRedirect(u); e != nil {
			return nil, e
		}
	}
	if in.AuthMethod == "" {
		in.AuthMethod = "none"
	}
	if in.AuthMethod != "none" && in.AuthMethod != "client_secret_basic" && in.AuthMethod != "client_secret_post" {
		return nil, errors.New("unsupported token endpoint authentication method")
	}
	if cimd && in.AuthMethod != "none" {
		return nil, errors.New("client metadata documents support public PKCE clients")
	}
	for _, g := range in.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			return nil, errors.New("unsupported grant type")
		}
	}
	for _, r := range in.ResponseTypes {
		if r != "code" {
			return nil, errors.New("unsupported response type")
		}
	}
	id := "client_" + secure.Random(18)
	if cimd {
		id = in.ClientID
	}
	c := Client{DefaultClient: fosite.DefaultClient{ID: id, RedirectURIs: in.RedirectURIs, GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, Scopes: []string{"db:read", "offline_access"}, Audience: []string{s.Resource}, Public: in.AuthMethod == "none"}, Name: in.Name, AuthMethod: in.AuthMethod, Revision: 1, CreatedAt: time.Now()}
	out := map[string]any{"client_id": id, "client_name": c.Name, "redirect_uris": c.RedirectURIs, "grant_types": c.GrantTypes, "response_types": c.ResponseTypes, "token_endpoint_auth_method": c.AuthMethod, "scope": "db:read offline_access", "client_id_issued_at": time.Now().Unix()}
	if !c.Public {
		secret := secure.Random(32)
		hash, e := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
		if e != nil {
			return nil, e
		}
		c.Secret = hash
		out["client_secret"] = secret
		out["client_secret_expires_at"] = 0
	}
	if cimd {
		var old Client
		_, err := s.storage.get(ctx, "client", id, &old)
		if err == nil {
			if old.Disabled {
				return nil, errors.New("client is disabled by the administrator")
			}
			if old.Name == c.Name && slices.Equal(old.RedirectURIs, c.RedirectURIs) {
				return out, nil
			}
			c.Revision, c.CreatedAt = old.Revision+1, old.CreatedAt
			return out, s.saveClient(ctx, c, false, !slices.Equal(old.RedirectURIs, c.RedirectURIs))
		}
		if !errors.Is(err, fosite.ErrNotFound) {
			return nil, err
		}
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(ctx); err != nil {
		return nil, err
	}
	if err := s.storage.put(ctx, "client", id, c, time.Time{}, ""); err != nil {
		return nil, err
	}
	return out, nil
}
func publicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	for _, block := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(block).Contains(address) {
			return false
		}
	}
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}
func fetchMetadata(ctx context.Context, raw string) (Registration, error) {
	var reg Registration
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Hostname() == "" {
		return reg, errors.New("client metadata URL requires public HTTPS")
	}
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(addr)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		if len(ips) == 0 {
			return nil, errors.New("client host did not resolve")
		}
		for _, a := range ips {
			if !publicIP(a.IP) {
				return nil, errors.New("client metadata cannot access private addresses")
			}
		}
		d := net.Dialer{Timeout: 5 * time.Second}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	tr := &http.Transport{DialContext: dial, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableCompression: true}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("client metadata redirects are denied") }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return reg, e
	}
	res, e := client.Do(req)
	if e != nil {
		return reg, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return reg, errors.New("client metadata fetch failed")
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 64*1024+1))
	if e != nil || len(b) > 64*1024 {
		return reg, errors.New("client metadata too large")
	}
	if e = json.Unmarshal(b, &reg); e != nil {
		return reg, e
	}
	if reg.ClientID != raw {
		return reg, errors.New("client metadata client_id must equal its URL")
	}
	return reg, nil
}
