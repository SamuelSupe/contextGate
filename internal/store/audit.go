package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"strings"
	"time"
)

type AuditFilter struct {
	Before                                            int64
	Agent, Source, Status, RequestID, EventKind, View string
	From, Until                                       time.Time
}

func (s *Store) Audit(a model.Audit) error {
	if a.EventKind == "" {
		a.EventKind = "query"
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// An OTLP cursor advances by ID. Assign IDs in commit order so a slower
	// concurrent insert cannot become visible behind an already exported cursor.
	if _, err = tx.Exec("LOCK TABLE audit IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO audit(at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code,request_id,native_code,preview,template_id,template_version,ontology_id,ontology_version,event_kind,resource_id,revision,changed_fields) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)", a.At.UnixMilli(), a.AgentID, a.SourceID, a.Operation, a.Fingerprint, a.ElapsedMS, a.Rows, a.ErrorCode, a.RequestID, a.NativeCode, a.Preview, a.TemplateID, a.TemplateVersion, a.OntologyID, a.OntologyVersion, a.EventKind, a.ResourceID, a.Revision, a.ChangedFields); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Audits(f AuditFilter, limit int) ([]model.Audit, error) {
	where, args := []string{"NOT (error_code='operation_pending' AND EXISTS (SELECT 1 FROM audit outcome WHERE outcome.request_id=audit.request_id AND outcome.id>audit.id))"}, []any{}
	for _, v := range []struct {
		clause  string
		value   any
		enabled bool
	}{
		{"id<$%d", f.Before, f.Before > 0},
		{"agent_id=$%d", f.Agent, f.Agent != ""},
		{"event_kind=$%d", f.EventKind, f.EventKind != ""},
		{"source_id=$%d", f.Source, f.Source != ""},
		{"request_id=$%d", f.RequestID, f.RequestID != ""},
		{"at>=$%d", f.From.UnixMilli(), !f.From.IsZero()},
		{"at<=$%d", f.Until.UnixMilli(), !f.Until.IsZero()},
	} {
		if v.enabled {
			args = append(args, v.value)
			where = append(where, fmt.Sprintf(v.clause, len(args)))
		}
	}
	switch f.View {
	case "client":
		where = append(where, "event_kind='query' AND preview=FALSE AND agent_id<>'admin'")
	case "preview":
		where = append(where, "event_kind='query' AND preview=TRUE")
	case "system":
		where = append(where, "event_kind='system'")
	}
	if f.Status == "success" {
		where = append(where, "error_code=''")
	}
	if f.Status == "error" {
		where = append(where, "error_code<>''")
	}
	args = append(args, limit)
	rows, err := s.DB.Query("SELECT id,at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code,request_id,native_code,preview,template_id,template_version,ontology_id,ontology_version,event_kind,resource_id,revision,changed_fields FROM audit WHERE "+strings.Join(where, " AND ")+fmt.Sprintf(" ORDER BY id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	return scanAudits(rows)
}

// AuditBatch reads a bounded export page; the extra character lets the encoder
// report truncation without loading unbounded caller-supplied identifiers.
func (s *Store) AuditBatch(ctx context.Context, after int64) ([]model.Audit, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,at,substr(agent_id,1,2049),substr(source_id,1,2049),substr(operation,1,2049),substr(fingerprint,1,2049),elapsed_ms,rows,substr(error_code,1,2049),substr(request_id,1,2049),substr(native_code,1,2049),preview,substr(template_id,1,2049),substr(template_version,1,2049),substr(ontology_id,1,2049),substr(ontology_version,1,2049),substr(event_kind,1,2049),substr(resource_id,1,2049),substr(revision,1,2049),substr(changed_fields,1,2049) FROM audit WHERE id>$1 ORDER BY id LIMIT 64`, after)
	if err != nil {
		return nil, err
	}
	return scanAudits(rows)
}

func scanAudits(rows *sql.Rows) ([]model.Audit, error) {
	defer rows.Close()
	out := []model.Audit{}
	for rows.Next() {
		var a model.Audit
		var at int64
		if err := rows.Scan(&a.ID, &at, &a.AgentID, &a.SourceID, &a.Operation, &a.Fingerprint, &a.ElapsedMS, &a.Rows, &a.ErrorCode, &a.RequestID, &a.NativeCode, &a.Preview, &a.TemplateID, &a.TemplateVersion, &a.OntologyID, &a.OntologyVersion, &a.EventKind, &a.ResourceID, &a.Revision, &a.ChangedFields); err != nil {
			return nil, err
		}
		a.At = time.UnixMilli(at)
		out = append(out, a)
	}
	return out, rows.Err()
}

type Activity struct {
	LastCall    *time.Time `json:"last_call,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
}

func (s *Store) AgentActivity() (map[string]Activity, error) {
	rows, err := s.DB.Query(`SELECT a.agent_id,a.at,a.error_code,
		(SELECT max(at) FROM audit WHERE agent_id=a.agent_id AND preview=FALSE AND error_code='')
		FROM audit a WHERE preview=FALSE AND id IN (SELECT max(id) FROM audit WHERE preview=FALSE GROUP BY agent_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Activity{}
	for rows.Next() {
		var id, code string
		var at int64
		var success *int64
		if err = rows.Scan(&id, &at, &code, &success); err != nil {
			return nil, err
		}
		t := time.UnixMilli(at)
		a := Activity{LastCall: &t, ErrorCode: code}
		if success != nil {
			t := time.UnixMilli(*success)
			a.LastSuccess = &t
		}
		out[id] = a
	}
	return out, rows.Err()
}
