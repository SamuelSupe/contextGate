package model

import "time"

type EvaluationQuestion struct {
	ID       string    `json:"id"`
	Revision int64     `json:"revision,string"`
	Name     string    `json:"name"`
	Question string    `json:"question"`
	Criteria string    `json:"criteria"`
	Client   string    `json:"client"`
	Created  time.Time `json:"created_at"`
}

type EvaluationStats struct {
	Calls             int64 `json:"calls"`
	Queries           int64 `json:"queries"`
	SuccessfulQueries int64 `json:"successful_queries"`
	Errors            int64 `json:"errors"`
	ElapsedMS         int64 `json:"elapsed_ms"`
	Templates         int64 `json:"template_calls"`
}

type EvaluationConfiguration struct {
	QueryRevision    int64  `json:"query_revision,string"`
	PublishedVersion int64  `json:"published_version,string"`
	AgentRevision    int64  `json:"agent_revision,string"`
	OntologyID       string `json:"ontology_id,omitempty"`
	OntologyVersion  int64  `json:"ontology_version,string,omitempty"`
}

type EvaluationCapture struct {
	State                string                  `json:"state"`
	Started              time.Time               `json:"started"`
	Until                *time.Time              `json:"until,omitempty"`
	Stats                *EvaluationStats        `json:"stats,omitempty"`
	Configuration        EvaluationConfiguration `json:"configuration"`
	ConfigurationChanged bool                    `json:"configuration_changed"`
	Verdict              string                  `json:"verdict"`
	Notes                string                  `json:"notes"`
}

type Evaluation struct {
	Mode         string                        `json:"mode,omitempty"`
	ID           string                        `json:"id"`
	Revision     int64                         `json:"revision,string"`
	SourceID     string                        `json:"source_id"`
	CaseID       string                        `json:"case_id,omitempty"`
	CaseRevision int64                         `json:"case_revision,string,omitempty"`
	Name         string                        `json:"name"`
	Question     string                        `json:"question"`
	Criteria     string                        `json:"criteria"`
	Client       string                        `json:"client"`
	AgentID      string                        `json:"agent_id"`
	AgentName    string                        `json:"agent_name"`
	Created      time.Time                     `json:"created_at"`
	Runs         map[string]*EvaluationCapture `json:"runs"`
}
