package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
)

func (s *Server) evaluationRoutes(mux *http.ServeMux) {
	for route, handler := range map[string]http.HandlerFunc{
		"GET /api/sources/{id}/evaluation/questions":                     s.listEvaluationQuestions,
		"POST /api/sources/{id}/evaluation/questions":                    s.saveEvaluationQuestion,
		"PUT /api/sources/{id}/evaluation/questions/{question}":          s.saveEvaluationQuestion,
		"DELETE /api/sources/{id}/evaluation/questions/{question}":       s.deleteEvaluationQuestion,
		"GET /api/sources/{id}/evaluation/history":                       s.listEvaluationHistory,
		"POST /api/sources/{id}/evaluation/history":                      s.createEvaluation,
		"GET /api/sources/{id}/evaluation/history/{evaluation}":          s.getEvaluation,
		"DELETE /api/sources/{id}/evaluation/history/{evaluation}":       s.deleteEvaluation,
		"POST /api/sources/{id}/evaluation/history/{evaluation}/capture": s.captureEvaluation,
		"PUT /api/sources/{id}/evaluation/history/{evaluation}/review":   s.reviewEvaluation,
	} {
		mux.HandleFunc(route, s.requireAdmin(handler))
	}
}

func (s *Server) listEvaluationQuestions(w http.ResponseWriter, r *http.Request) {
	s.listEvaluationRecords(w, r, true)
}
func (s *Server) listEvaluationHistory(w http.ResponseWriter, r *http.Request) {
	s.listEvaluationRecords(w, r, false)
}
func (s *Server) listEvaluationRecords(w http.ResponseWriter, r *http.Request, questions bool) {
	source := r.PathValue("id")
	if _, err := s.Store.Source(source); err != nil {
		semanticFailure(w, model.Fail("not_found", "Data source not found"))
		return
	}
	var cursor int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var err error
		cursor, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cursor <= 0 {
			semanticFailure(w, model.Fail("invalid_input", "Invalid history cursor"))
			return
		}
	}
	filter := store.EvaluationFilter{Search: strings.TrimSpace(r.URL.Query().Get("search")), Agent: r.URL.Query().Get("agent")}
	if len(filter.Search) > 480 || len(filter.Agent) > 200 {
		semanticFailure(w, model.Fail("invalid_input", "History filter is too long"))
		return
	}
	for key, target := range map[string]*time.Time{"from": &filter.From, "until": &filter.Until} {
		if raw := r.URL.Query().Get(key); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				semanticFailure(w, model.Fail("invalid_input", "History dates must use RFC3339 timestamps"))
				return
			}
			*target = parsed
		}
	}
	if !filter.From.IsZero() && !filter.Until.IsZero() && !filter.From.Before(filter.Until) {
		semanticFailure(w, model.Fail("invalid_input", "The end date must follow the start date"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	page, err := s.Store.SearchEvaluations(ctx, source, questions, cursor, filter)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, page)
}

func validateEvaluationQuestion(name, question, criteria, client string) error {
	if strings.TrimSpace(name) == "" || len(name) > 480 || strings.TrimSpace(question) == "" || len(question) > 16000 || strings.TrimSpace(criteria) == "" || len(criteria) > 16000 || len(client) > 2000 {
		return model.Fail("invalid_input", "Enter a name, business question and acceptance criteria within the field limits")
	}
	return nil
}

func (s *Server) saveEvaluationQuestion(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64  `json:"revision,string"`
		Name     string `json:"name"`
		Question string `json:"question"`
		Criteria string `json:"criteria"`
		Client   string `json:"client"`
	}
	if err := decode(r, &in); err != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid question fields"))
		return
	}
	if err := validateEvaluationQuestion(in.Name, in.Question, in.Criteria, in.Client); err != nil {
		semanticFailure(w, err)
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	source, id := r.PathValue("id"), r.PathValue("question")
	if _, err := s.Store.Source(source); err != nil {
		semanticFailure(w, model.Fail("not_found", "Data source not found"))
		return
	}
	v := model.EvaluationQuestion{ID: id, Revision: in.Revision + 1, Name: in.Name, Question: in.Question, Criteria: in.Criteria, Client: in.Client, Created: time.Now().UTC()}
	if id == "" {
		if in.Revision != 0 {
			semanticFailure(w, model.Fail("invalid_input", "A new question cannot specify a revision"))
			return
		}
		v.ID = "case_" + secure.Random(16)
	} else {
		old, err := s.Store.EvaluationQuestion(source, id)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		if old.Revision != in.Revision {
			semanticFailure(w, model.Fail("conflict", "Question changed; reload before saving"))
			return
		}
		v.Created = old.Created
	}
	if err := s.Store.SaveEvaluationQuestion(source, v, in.Revision); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) getEvaluation(w http.ResponseWriter, r *http.Request) {
	v, err := s.Store.Evaluation(r.PathValue("id"), r.PathValue("evaluation"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) deleteEvaluationQuestion(w http.ResponseWriter, r *http.Request) {
	s.deleteEvaluationRecord(w, r, true)
}
func (s *Server) deleteEvaluation(w http.ResponseWriter, r *http.Request) {
	s.deleteEvaluationRecord(w, r, false)
}
func (s *Server) deleteEvaluationRecord(w http.ResponseWriter, r *http.Request, questions bool) {
	var in struct {
		Revision int64 `json:"revision,string"`
	}
	if decode(r, &in) != nil || in.Revision < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Current revision is required"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	id := r.PathValue("question")
	if !questions {
		id = r.PathValue("evaluation")
		v, err := s.Store.Evaluation(r.PathValue("id"), id)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		for _, run := range v.Runs {
			if run.State == "capturing" {
				semanticFailure(w, model.Fail("conflict", "Collect or abandon the active capture before deleting"))
				return
			}
		}
	}
	if err := s.Store.DeleteEvaluation(r.PathValue("id"), id, questions, in.Revision); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
