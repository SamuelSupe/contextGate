package model

import "time"

type HealthConfig struct {
	Revision        int64 `json:"revision,string"`
	Enabled         bool  `json:"enabled"`
	IntervalMinutes int   `json:"interval_minutes"`
}

type StructureChange struct {
	Column string `json:"column"`
	Change string `json:"change"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type StructureHealth struct {
	Changes       []StructureChange `json:"changes,omitempty"`
	DetailLimited bool              `json:"detail_limited,omitempty"`
	Namespace     string            `json:"namespace"`
	Object        string            `json:"object"`
	Status        string            `json:"status"`
}

type SourceHealth struct {
	BaselineColumns    map[string][]Column `json:"-"`
	CurrentColumns     map[string][]Column `json:"-"`
	SourceID           string              `json:"source_id"`
	ConnectionRevision int64               `json:"connection_revision,string"`
	PublishedVersion   int64               `json:"published_version,string"`
	CheckedAt          time.Time           `json:"checked_at"`
	Probe              Probe               `json:"probe"`
	Structure          []StructureHealth   `json:"structure"`
	CoverageLimited    bool                `json:"coverage_limited"`
	Baseline           map[string]string   `json:"-"`
	Current            map[string]string   `json:"-"`
}
