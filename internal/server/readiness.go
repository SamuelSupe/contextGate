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

type templateActivity struct {
	TemplateID      string       `json:"template_id"`
	Version         string       `json:"execution_version"`
	AgentID         string       `json:"agent_id"`
	SuccessfulCalls int64        `json:"successful_calls"`
	Calls           []clientCall `json:"recent_calls"`
}

type clientCall struct {
	AgentID   string    `json:"agent_id"`
	RequestID string    `json:"request_id"`
	At        time.Time `json:"at"`
}

type templateReadiness struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"execution_version"`
	Status      string   `json:"status"`
	Executable  bool     `json:"executable"`
	Concepts    []string `json:"concept_refs"`
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
	ActivitySince       time.Time           `json:"activity_since"`
	ClientQueries       int64               `json:"client_queries"`
	TemplateActivity    *templateActivity   `json:"template_activity,omitempty"`
}

func (s *Server) sourceReadiness(w http.ResponseWriter, r *http.Request) {
	// A readiness response describes a single configuration snapshot. It never
	// probes user databases and never substitutes for execution-time authorization.
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
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
	out.ActivitySince = out.CheckedAt.Add(-30 * 24 * time.Hour)
	for _, en := range st.Published.Entries {
		if en.Template == nil {
			continue
		}
		v := s.Engine.PublishedTemplateValidation(src, en)
		out.Templates = append(out.Templates, templateReadiness{ID: en.ID, Name: en.Name, Description: catalogText(en.Description, 256), Version: en.Template.ExecutionVersion, Status: v.Status, Executable: v.Valid, Concepts: en.Template.ConceptRefs})
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err = s.Store.DB.QueryRowContext(ctx, `SELECT count(*),max(at) FROM audit WHERE source_id=$1 AND agent_id<>'admin' AND event_kind='query' AND actor_type='query_agent' AND preview=FALSE AND error_code='' AND at >= $2 AND at <= $3 AND (operation LIKE 'query_%' OR operation='execute_query_template')`, src.ID, out.ActivitySince.UnixMilli(), out.CheckedAt.UnixMilli()).Scan(&out.ClientQueries, &last)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if last != nil {
		at := time.UnixMilli(*last)
		out.LastQuery = &at
	}
	if id := r.URL.Query().Get("template_id"); id != "" {
		agent := r.URL.Query().Get("agent_id")
		if len(id) > 96 || len(agent) > 256 {
			semanticFailure(w, model.Fail("invalid_input", "Invalid query activity scope"))
			return
		}
		activity := &templateActivity{TemplateID: id, AgentID: agent, Calls: []clientCall{}}
		found := false
		for _, en := range st.Draft.Entries {
			found = found || en.ID == id && en.Template != nil
		}
		for _, en := range st.Published.Entries {
			if en.ID == id && en.Template != nil {
				found, activity.Version = true, en.Template.ExecutionVersion
			}
		}
		if !found {
			semanticFailure(w, model.Fail("not_found", "Query template not found"))
			return
		}
		// A source-wide success must not activate an unrelated template, version
		// or Agent. These are historical calls, never proof of current credentials.
		if activity.Version != "" {
			rows, err := s.Store.DB.QueryContext(ctx, `SELECT count(*) OVER(),agent_id,request_id,at FROM audit
 WHERE source_id=$1 AND template_id=$2 AND template_version=$3
 AND ($4='' OR agent_id=$4) AND agent_id<>'admin' AND preview=FALSE
 AND event_kind='query' AND actor_type='query_agent' AND operation='execute_query_template' AND error_code='' AND at >= $5 AND at <= $6
 ORDER BY at DESC,id DESC LIMIT 10`, src.ID, id, activity.Version, agent, out.ActivitySince.UnixMilli(), out.CheckedAt.UnixMilli())
			if err != nil {
				semanticFailure(w, err)
				return
			}
			defer rows.Close()
			for rows.Next() {
				var call clientCall
				var at int64
				if err := rows.Scan(&activity.SuccessfulCalls, &call.AgentID, &call.RequestID, &at); err != nil {
					semanticFailure(w, err)
					return
				}
				call.At = time.UnixMilli(at)
				activity.Calls = append(activity.Calls, call)
			}
			if err := rows.Err(); err != nil {
				semanticFailure(w, err)
				return
			}
		}
		out.TemplateActivity = activity
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
