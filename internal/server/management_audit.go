package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

func managementOperation(r *http.Request) string {
	if r.Method == "GET" || r.Method == "HEAD" {
		return ""
	}
	path := strings.TrimPrefix(r.Pattern, r.Method+" ")
	switch path {
	case "/api/sources", "/api/sources/{id}":
		return "source." + map[string]string{"POST": "create", "PUT": "update", "DELETE": "delete"}[r.Method]
	case "/api/agents", "/api/agents/{id}":
		return "agent." + map[string]string{"POST": "create", "PUT": "update", "DELETE": "revoke"}[r.Method]
	case "/api/agents/{id}/token":
		return "agent.rotate_token"
	case "/api/configuration-agents":
		return "configuration_agent.create"
	case "/api/configuration-agents/{id}":
		return "configuration_agent.revoke"
	case "/api/password":
		return "administrator.change_password"
	case "/api/settings/audit-export":
		return "settings.audit_export"
	case "/api/settings/health":
		return "settings.health"
	case "/api/health/{id}/baseline":
		return "health.accept_baseline"
	case "/api/oauth/consent":
		return "oauth.consent"
	}
	if strings.HasPrefix(path, "/api/oauth/clients") {
		return "oauth.client." + strings.ToLower(r.Method)
	}
	if strings.HasPrefix(path, "/api/ontologies") {
		for _, read := range []string{"/validate", "/diff", "/preview"} {
			if strings.HasSuffix(path, read) {
				return ""
			}
		}
		return "ontology." + strings.ToLower(r.Method) + "." + strings.TrimPrefix(path, "/api/ontologies")
	}
	if strings.HasPrefix(path, "/api/sources/{id}/semantics") {
		tail := strings.TrimPrefix(path, "/api/sources/{id}/semantics")
		switch tail {
		case "", "/import", "/import-structure", "/publish", "/discard", "/restore", "/entries/{entry}", "/versions/{version}":
			return "semantics." + strings.ToLower(r.Method) + strings.ReplaceAll(tail, "/", ".")
		}
	}
	return ""
}

// Record intent before changing configuration. A crash or failed outcome write
// leaves a visible pending event, rather than an unaudited successful mutation.
// Only fixed field categories and revision numbers are retained, never values.
func (s *Server) adminChange(next http.HandlerFunc, w http.ResponseWriter, r *http.Request) {
	operation := managementOperation(r)
	if operation == "" {
		next(w, r)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 400, model.Fail("invalid_input", "Request body exceeds its limit"))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	input := map[string]json.RawMessage{}
	json.Unmarshal(raw, &input)
	fields := []string{}
	for key := range input {
		switch key {
		case "password", "current_password", "token", "headers", "clear_headers", "clear_secret", "auth_mode":
			fields = append(fields, "credentials")
		case "snapshot", "entry", "draft", "definition", "http_api":
			fields = append(fields, "definition")
		case "name", "enabled", "sources", "expires_at", "host", "port", "database", "username", "path", "tls_mode", "ca_cert", "options", "limits", "query_access_mode", "version", "interval_minutes", "client_name", "redirect_uris", "grant_types", "response_types", "scope", "token_endpoint_auth_method", "agent_id", "allow", "archived", "endpoint", "protocol", "service_name", "insecure":
			fields = append(fields, key)
		}
	}
	slices.Sort(fields)
	fields = slices.Compact(fields)
	a := model.Audit{At: time.Now().UTC(), AgentID: "admin", EventKind: "management", Operation: operation, ResourceID: r.PathValue("id"), RequestID: secure.Random(16), ChangedFields: strings.Join(fields, ","), ErrorCode: "operation_pending"}
	if strings.HasPrefix(r.Pattern, r.Method+" /api/sources/") {
		a.SourceID = r.PathValue("id")
		if entry := r.PathValue("entry"); entry != "" {
			a.ResourceID += ":entry:" + entry
		}
		if version := r.PathValue("version"); version != "" {
			a.ResourceID += ":publication:" + version
		}
	}
	if strings.HasPrefix(r.Pattern, r.Method+" /api/ontologies/") {
		a.OntologyID = r.PathValue("ontology")
		a.ResourceID = a.OntologyID
		a.OntologyVersion = r.PathValue("version")
	}
	if v, ok := input["revision"]; ok {
		var revision string
		if json.Unmarshal(v, &revision) == nil {
			if _, err := strconv.ParseInt(revision, 10, 64); err == nil {
				a.Revision = revision
			}
		}
	}
	if err := s.Store.Audit(a); err != nil {
		fail(w, 503, model.Fail("audit_unavailable", "Configuration was not changed because the audit log is unavailable"))
		return
	}
	output := httptest.NewRecorder()
	next(output, r)
	a.ErrorCode = ""
	a.ElapsedMS = time.Since(a.At).Milliseconds()
	if output.Code >= 400 {
		a.ErrorCode = "operation_failed"
	}
	// Decode only a small response envelope; credential fields are ignored.
	var envelope struct {
		ID       string       `json:"id"`
		ClientID string       `json:"client_id"`
		Revision string       `json:"revision"`
		Agent    *model.Agent `json:"agent"`
		Error    *model.Error `json:"error"`
	}
	if json.Unmarshal(output.Body.Bytes(), &envelope) == nil {
		if envelope.ID != "" {
			a.ResourceID = envelope.ID
		}
		if envelope.ClientID != "" {
			a.ResourceID = envelope.ClientID
		}
		if envelope.Revision != "" {
			a.Revision = envelope.Revision
		}
		if envelope.Agent != nil {
			a.ResourceID = envelope.Agent.ID
			a.Revision = strconv.FormatInt(envelope.Agent.Revision, 10)
		}
		if envelope.Error != nil {
			a.ErrorCode = envelope.Error.Code
		}
	}
	if strings.HasPrefix(operation, "ontology.") {
		a.OntologyID = a.ResourceID
	}
	if operation == "source.create" || operation == "health.accept_baseline" {
		a.SourceID = a.ResourceID
	}
	if err := s.Store.Audit(a); err != nil {
		log.Print("Management operation outcome could not be recorded; its audit intent remains pending")
		output.Header().Set("X-Audit-Status", "pending")
	}
	for key, values := range output.Header() {
		w.Header()[key] = values
	}
	w.WriteHeader(output.Code)
	w.Write(output.Body.Bytes())
}
