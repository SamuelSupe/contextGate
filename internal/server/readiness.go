package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
)

type templateReadiness struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"execution_version"`
	Status     string   `json:"status"`
	Executable bool     `json:"executable"`
	Concepts   []string `json:"concept_refs"`
}

type sourceReadiness struct {
	SourceID            string              `json:"source_id"`
	PublishedVersion    string              `json:"published_version"`
	DraftRevision       string              `json:"draft_revision"`
	Templates           []templateReadiness `json:"templates"`
	ExecutableTemplates int                 `json:"executable_templates"`
	ActiveAgents        []string            `json:"active_agents"`
	Ontology            *ontology.Binding   `json:"ontology,omitempty"`
	DraftOntology       *ontology.Binding   `json:"draft_ontology,omitempty"`
	LastQuery           *time.Time          `json:"last_query,omitempty"`
	QueryRevision       string              `json:"query_revision"`
	CheckedAt           time.Time           `json:"checked_at"`
}

func (s *Server) sourceReadiness(w http.ResponseWriter, r *http.Request) {
	// A readiness response describes a single configuration snapshot. It never
	// probes user databases and never substitutes for execution-time authorization.
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	src, st, err := s.semanticState(r.PathValue("id"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	agents, err := s.Store.Agents()
	if err != nil {
		semanticFailure(w, err)
		return
	}
	out := sourceReadiness{SourceID: src.ID, PublishedVersion: strconv.FormatInt(st.PublishedVersion, 10), DraftRevision: strconv.FormatInt(st.Revision, 10), QueryRevision: strconv.FormatInt(src.ExecutionRevision(), 10), Templates: []templateReadiness{}, ActiveAgents: []string{}, Ontology: st.Published.Ontology, DraftOntology: st.Draft.Ontology, CheckedAt: time.Now().UTC()}
	for _, en := range st.Published.Entries {
		if en.Template == nil {
			continue
		}
		v := s.Engine.PublishedTemplateValidation(src, en)
		out.Templates = append(out.Templates, templateReadiness{ID: en.ID, Name: en.Name, Version: en.Template.ExecutionVersion, Status: v.Status, Executable: v.Valid, Concepts: en.Template.ConceptRefs})
		if v.Valid {
			out.ExecutableTemplates++
		}
	}
	for _, a := range agents {
		if a.Enabled && a.RevokedAt == nil && a.ExpiresAt.After(out.CheckedAt) && slices.Contains(a.Sources, src.ID) {
			out.ActiveAgents = append(out.ActiveAgents, a.ID)
		}
	}
	var last *int64
	err = s.Store.DB.QueryRowContext(r.Context(), `SELECT max(at) FROM audit WHERE source_id=$1 AND agent_id<>'admin' AND preview=FALSE AND error_code='' AND (operation LIKE 'query_%' OR operation='execute_query_template')`, src.ID).Scan(&last)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if last != nil {
		at := time.UnixMilli(*last)
		out.LastQuery = &at
	}
	if r.URL.Query().Get("summary") == "1" {
		out.Templates = []templateReadiness{}
		out.Ontology = nil
		out.DraftOntology = nil
	}
	write(w, 200, out)
}

type evaluationStats = model.EvaluationStats

func (s *Server) evaluationActivity(w http.ResponseWriter, r *http.Request) {
	id, agentID := r.PathValue("id"), r.URL.Query().Get("agent_id")
	if _, err := s.Store.Source(id); err != nil {
		semanticFailure(w, model.Fail("not_found", "Data source not found"))
		return
	}
	if agentID == "" || agentID == "admin" {
		semanticFailure(w, model.Fail("invalid_input", "Choose an Agent for a real client evaluation"))
		return
	}
	if _, err := s.Store.Agent(agentID); err != nil {
		semanticFailure(w, model.Fail("not_found", "Agent not found"))
		return
	}
	until := time.Now().UTC().Truncate(time.Millisecond)
	from := until
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || parsed.After(until) || until.Sub(parsed) > 24*time.Hour {
			semanticFailure(w, model.Fail("invalid_input", "Evaluation window must be within the last 24 hours"))
			return
		}
		from = parsed
	}
	stats, err := s.evaluationStats(r.Context(), id, agentID, from, until)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, map[string]any{"from": from, "until": until, "stats": stats})
}

func (s *Server) evaluationStats(ctx context.Context, source, agent string, from, until time.Time) (evaluationStats, error) {
	stats := evaluationStats{}
	// End-exclusive windows avoid counting a boundary event in both runs.
	// Only completed audit records for this Agent/source are included, never previews.
	err := s.Store.DB.QueryRowContext(ctx, `SELECT count(*),
 count(*) FILTER (WHERE operation LIKE 'query_%' OR operation='execute_query_template'),
 count(*) FILTER (WHERE error_code='' AND (operation LIKE 'query_%' OR operation='execute_query_template')),
 count(*) FILTER (WHERE error_code<>''), coalesce(sum(elapsed_ms),0),
 count(*) FILTER (WHERE operation='execute_query_template')
 FROM audit WHERE source_id=$1 AND agent_id=$2 AND preview=FALSE AND at>=$3 AND at<$4`, source, agent, from.UnixMilli(), until.UnixMilli()).Scan(&stats.Calls, &stats.Queries, &stats.SuccessfulQueries, &stats.Errors, &stats.ElapsedMS, &stats.Templates)
	return stats, err
}
