package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/ory/fosite"
	"golang.org/x/crypto/bcrypt"
)

type clientView struct {
	ID               string    `json:"client_id"`
	Name             string    `json:"client_name"`
	RedirectURIs     []string  `json:"redirect_uris"`
	AuthMethod       string    `json:"token_endpoint_auth_method"`
	Enabled          bool      `json:"enabled"`
	Revision         int64     `json:"revision,string"`
	CreatedAt        time.Time `json:"created_at"`
	MetadataDocument bool      `json:"metadata_document"`
}

func publicClient(c Client) clientView {
	return clientView{c.ID, c.Name, c.RedirectURIs, c.AuthMethod, !c.Disabled, c.Revision, c.CreatedAt, strings.HasPrefix(c.ID, "https://")}
}

func clientError(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": &model.Error{Code: code, Message: message}})
}

func (s *Server) ListClients(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.DB.QueryContext(r.Context(), "SELECT id,value FROM oauth WHERE kind='client' ORDER BY id")
	if err != nil {
		clientError(w, 500, "storage_unavailable", "Client storage is unavailable.")
		return
	}
	defer rows.Close()
	out := []clientView{}
	for rows.Next() {
		var id, value string
		if err = rows.Scan(&id, &value); err != nil {
			break
		}
		var plain []byte
		plain, err = s.Store.Vault.Open(value, "oauth:client:"+id)
		if err != nil {
			break
		}
		var c Client
		if err = json.Unmarshal(plain, &c); err != nil {
			break
		}
		out = append(out, publicClient(c))
	}
	if err != nil || rows.Err() != nil {
		clientError(w, 500, "storage_unavailable", "Client storage is unavailable.")
		return
	}
	write(w, 200, out)
}

type clientEdit struct {
	Name         string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	Enabled      bool     `json:"enabled"`
	Revision     int64    `json:"revision,string"`
}

func decodeClientEdit(w http.ResponseWriter, r *http.Request) (clientEdit, bool) {
	var in clientEdit
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		clientError(w, 400, "invalid_input", "Invalid client configuration.")
		return in, false
	}
	return in, true
}

func (s *Server) clientForEdit(w http.ResponseWriter, r *http.Request, in clientEdit) (Client, bool) {
	var c Client
	if _, err := s.storage.get(r.Context(), "client", r.PathValue("id"), &c); err != nil {
		clientError(w, 404, "client_not_found", "Client not found.")
		return c, false
	}
	if in.Revision != c.Revision {
		clientError(w, 409, "conflict", "Client changed. Reload the client before saving.")
		return c, false
	}
	return c, true
}

func (s *Server) UpdateClient(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeClientEdit(w, r)
	if !ok {
		return
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	c, ok := s.clientForEdit(w, r, in)
	if !ok {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 120 || len(in.RedirectURIs) < 1 || len(in.RedirectURIs) > 10 {
		clientError(w, 400, "invalid_input", "A name and 1–10 redirect URIs are required.")
		return
	}
	for _, redirect := range in.RedirectURIs {
		if err := validateRedirect(redirect); err != nil {
			clientError(w, 400, "invalid_input", err.Error())
			return
		}
	}
	if strings.HasPrefix(c.ID, "https://") && (in.Name != c.Name || !slices.Equal(in.RedirectURIs, c.RedirectURIs)) {
		clientError(w, 400, "invalid_input", "Update the client metadata document to change its name or redirect URIs.")
		return
	}
	invalidate := c.Disabled != !in.Enabled || !slices.Equal(c.RedirectURIs, in.RedirectURIs)
	c.Name, c.RedirectURIs, c.Disabled = in.Name, in.RedirectURIs, !in.Enabled
	c.Revision++
	if err := s.saveClient(r.Context(), c, false, invalidate); err != nil {
		clientError(w, 500, "storage_unavailable", "Could not update the client.")
		return
	}
	write(w, 200, map[string]any{"client": publicClient(c)})
}

func (s *Server) RotateClientSecret(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeClientEdit(w, r)
	if !ok {
		return
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	c, ok := s.clientForEdit(w, r, in)
	if !ok {
		return
	}
	if c.Public {
		clientError(w, 400, "invalid_input", "Public PKCE clients do not have a client secret.")
		return
	}
	secret := secure.Random(32)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		clientError(w, 500, "storage_unavailable", "Could not create the client secret.")
		return
	}
	c.Secret, c.Revision = hash, c.Revision+1
	if err = s.saveClient(r.Context(), c, false, true); err != nil {
		clientError(w, 500, "storage_unavailable", "Could not rotate the client secret.")
		return
	}
	write(w, 200, map[string]any{"client": publicClient(c), "client_secret": secret})
}

func (s *Server) DeleteClient(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeClientEdit(w, r)
	if !ok {
		return
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	c, ok := s.clientForEdit(w, r, in)
	if !ok {
		return
	}
	if err := s.saveClient(r.Context(), c, true, true); err != nil {
		clientError(w, 500, "storage_unavailable", "Could not delete the client.")
		return
	}
	write(w, 200, map[string]any{"ok": true})
}

// Client changes, outstanding OAuth requests and Agent grants commit together.
// Reading stored requests also finds grants created before Agent.client_id existed.
// Callers hold tokenMu and clientMu, so token exchange cannot race this invalidation.
func (s *Server) saveClient(ctx context.Context, c Client, remove, invalidate bool) error {
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	type record struct{ kind, id string }
	records := []record{}
	subjects := map[string]bool{}
	if invalidate {
		rows, err := s.Store.DB.QueryContext(ctx, "SELECT kind,id,value FROM oauth WHERE kind IN ('consent','code','pkce','access','refresh')")
		if err != nil {
			return err
		}
		err = func() error {
			defer rows.Close()
			for rows.Next() {
				var kind, id, value string
				if err := rows.Scan(&kind, &id, &value); err != nil {
					return err
				}
				plain, err := s.Store.Vault.Open(value, "oauth:"+kind+":"+id)
				if err != nil {
					return err
				}
				clientID, subject := "", ""
				if kind == "consent" {
					var form url.Values
					if err := json.Unmarshal(plain, &form); err != nil {
						return err
					}
					clientID = form.Get("client_id")
				} else {
					req := fosite.NewRequest()
					req.Session = &fosite.DefaultSession{}
					if err := json.Unmarshal(plain, req); err != nil {
						return err
					}
					if req.GetClient() == nil || req.GetSession() == nil {
						return errors.New("invalid stored OAuth request")
					}
					clientID, subject = req.GetClient().GetID(), req.GetSession().GetSubject()
				}
				if clientID == c.ID {
					records = append(records, record{kind, id})
					subjects[subject] = true
				}
			}
			return rows.Err()
		}()
		if err != nil {
			return err
		}
	}
	agents, err := s.Store.Agents()
	if err != nil {
		return err
	}
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if remove {
		_, err = tx.ExecContext(ctx, "DELETE FROM oauth WHERE kind='client' AND id=$1", c.ID)
	} else {
		var b []byte
		b, err = json.Marshal(c)
		if err == nil {
			_, err = tx.ExecContext(ctx, "UPDATE oauth SET value=$1 WHERE kind='client' AND id=$2", s.Store.Vault.Seal(b, "oauth:client:"+c.ID), c.ID)
		}
	}
	if err != nil {
		return err
	}
	for _, item := range records {
		if _, err = tx.ExecContext(ctx, "DELETE FROM oauth WHERE kind=$1 AND id=$2", item.kind, item.id); err != nil {
			return err
		}
	}
	affected := []string{}
	now := time.Now()
	for _, a := range agents {
		if !invalidate || a.AuthType != "oauth" || a.ClientID != c.ID && !subjects[a.ID] {
			continue
		}
		a.Enabled, a.RevokedAt, a.Revision, a.ClientID = false, &now, a.Revision+1, c.ID
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE agents SET value=$1,token_hash=NULL WHERE id=$2", string(b), a.ID); err != nil {
			return err
		}
		affected = append(affected, a.ID)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, id := range affected {
		s.Engine.InvalidateAgent(id)
	}
	return nil
}
