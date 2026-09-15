package server

import (
	"net/http"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
)

func (s *Server) evaluationConfiguration(source, agent string) (model.EvaluationConfiguration, error) {
	src, st, err := s.semanticState(source)
	if err != nil {
		return model.EvaluationConfiguration{}, err
	}
	a, err := s.Store.Agent(agent)
	if err != nil {
		return model.EvaluationConfiguration{}, err
	}
	v := model.EvaluationConfiguration{QueryRevision: src.ExecutionRevision(), PublishedVersion: st.PublishedVersion, AgentRevision: a.Revision}
	if st.Published.Ontology != nil {
		v.OntologyID, v.OntologyVersion = st.Published.Ontology.OntologyID, st.Published.Ontology.Version
	}
	return v, nil
}

func (s *Server) beginEvaluationCapture(v *model.Evaluation, kind string) error {
	if v.Mode == "single" && kind != "guided" {
		return model.Fail("invalid_input", "A single answer check only captures the guided query")
	}
	if kind != "baseline" && kind != "guided" {
		return model.Fail("invalid_input", "Choose baseline or guided")
	}
	if v.Runs[kind] != nil {
		return model.Fail("conflict", "This capture already exists; start a new evaluation to repeat it")
	}
	for _, run := range v.Runs {
		if run.State == "capturing" {
			return model.Fail("conflict", "Finish the active capture first")
		}
	}
	if _, err := s.Engine.Authorize(model.Principal{AgentID: v.AgentID}, v.SourceID); err != nil {
		return err
	}
	configuration, err := s.evaluationConfiguration(v.SourceID, v.AgentID)
	if err != nil {
		return err
	}
	v.Runs[kind] = &model.EvaluationCapture{State: "capturing", Started: time.Now().UTC().Truncate(time.Millisecond), Configuration: configuration, Verdict: "unrated"}
	return nil
}

func (s *Server) createEvaluation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mode         string `json:"mode"`
		CaseID       string `json:"case_id"`
		CaseRevision int64  `json:"case_revision,string"`
		Name         string `json:"name"`
		Question     string `json:"question"`
		Criteria     string `json:"criteria"`
		Client       string `json:"client"`
		AgentID      string `json:"agent_id"`
		Kind         string `json:"kind"`
	}
	if decode(r, &in) != nil {
		semanticFailure(w, model.Fail("invalid_input", "Invalid evaluation fields; measurements are collected by the server"))
		return
	}
	if in.Mode != "" && in.Mode != "single" && in.Mode != "comparison" {
		semanticFailure(w, model.Fail("invalid_input", "Choose single or comparison evaluation"))
		return
	}
	if err := validateEvaluationQuestion(in.Name, in.Question, in.Criteria, in.Client); err != nil {
		semanticFailure(w, err)
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	v := model.Evaluation{ID: "eval_" + secure.Random(16), Revision: 1, SourceID: r.PathValue("id"), CaseID: in.CaseID, CaseRevision: in.CaseRevision, Name: in.Name, Question: in.Question, Criteria: in.Criteria, Client: in.Client, AgentID: in.AgentID, Created: time.Now().UTC(), Runs: map[string]*model.EvaluationCapture{}}
	v.Mode = in.Mode
	if in.CaseID != "" {
		q, err := s.Store.EvaluationQuestion(v.SourceID, in.CaseID)
		if err != nil {
			semanticFailure(w, err)
			return
		}
		if q.Revision != in.CaseRevision || q.Question != in.Question || q.Criteria != in.Criteria {
			semanticFailure(w, model.Fail("conflict", "Saved question changed; save or reload it before starting"))
			return
		}
	} else {
		v.CaseRevision = 0
	}
	if err := s.beginEvaluationCapture(&v, in.Kind); err != nil {
		semanticFailure(w, err)
		return
	}
	a, err := s.Store.Agent(v.AgentID)
	if err != nil {
		semanticFailure(w, err)
		return
	}
	v.AgentName = a.Name
	if err := s.Store.SaveEvaluation(v, 0); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) captureEvaluation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64  `json:"revision,string"`
		Kind     string `json:"kind"`
		Action   string `json:"action"`
	}
	if decode(r, &in) != nil || in.Revision < 1 {
		semanticFailure(w, model.Fail("invalid_input", "Current revision and capture action are required"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	v, err := s.Store.Evaluation(r.PathValue("id"), r.PathValue("evaluation"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if in.Revision != v.Revision {
		semanticFailure(w, model.Fail("conflict", "Evaluation changed; reload to continue"))
		return
	}
	if in.Action == "start" {
		if err = s.beginEvaluationCapture(&v, in.Kind); err != nil {
			semanticFailure(w, err)
			return
		}
	} else {
		run := v.Runs[in.Kind]
		if run == nil || run.State != "capturing" {
			semanticFailure(w, model.Fail("conflict", "No active capture for this run"))
			return
		}
		until := time.Now().UTC().Truncate(time.Millisecond)
		switch in.Action {
		case "collect":
			if until.Sub(run.Started) > 24*time.Hour {
				semanticFailure(w, model.Fail("invalid_input", "Capture expired after 24 hours; abandon it and start a new evaluation"))
				return
			}
			stats, err := s.evaluationStats(r.Context(), v.SourceID, v.AgentID, run.Started, until)
			if err != nil {
				semanticFailure(w, err)
				return
			}
			configuration, err := s.evaluationConfiguration(v.SourceID, v.AgentID)
			if err != nil {
				semanticFailure(w, err)
				return
			}
			run.Stats, run.State = &stats, "completed"
			run.ConfigurationChanged = configuration != run.Configuration
		case "abandon":
			run.State = "abandoned"
		default:
			semanticFailure(w, model.Fail("invalid_input", "Choose start, collect or abandon"))
			return
		}
		run.Until = &until
	}
	v.Revision++
	if err := s.Store.SaveEvaluation(v, in.Revision); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) reviewEvaluation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int64 `json:"revision,string"`
		Reviews  map[string]struct {
			Verdict string `json:"verdict"`
			Notes   string `json:"notes"`
		} `json:"reviews"`
	}
	if decode(r, &in) != nil || in.Revision < 1 || len(in.Reviews) == 0 {
		semanticFailure(w, model.Fail("invalid_input", "Current revision and reviews are required"))
		return
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	if err := model.CheckConfigurationContext(r.Context()); err != nil {
		fail(w, 401, err)
		return
	}
	v, err := s.Store.Evaluation(r.PathValue("id"), r.PathValue("evaluation"))
	if err != nil {
		semanticFailure(w, err)
		return
	}
	if v.Revision != in.Revision {
		semanticFailure(w, model.Fail("conflict", "Evaluation changed; reload before reviewing"))
		return
	}
	for kind, review := range in.Reviews {
		run := v.Runs[kind]
		if run == nil || run.State != "completed" || len(review.Notes) > 16000 {
			semanticFailure(w, model.Fail("invalid_input", "Only completed captures can be reviewed; notes must fit the field limit"))
			return
		}
		switch review.Verdict {
		case "unrated", "correct", "partial", "incorrect":
		default:
			semanticFailure(w, model.Fail("invalid_input", "Invalid answer assessment"))
			return
		}
		run.Verdict, run.Notes = review.Verdict, review.Notes
	}
	v.Revision++
	if err := s.Store.SaveEvaluation(v, in.Revision); err != nil {
		semanticFailure(w, err)
		return
	}
	write(w, 200, v)
}
