package store

import (
	"context"
	"database/sql"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"strings"
	"time"
)

type AuditFilter struct {
	Before                           int64
	Agent, Source, Status, RequestID string
	From, Until                      time.Time
}

func (s *Store) Audit(a model.Audit) error {
	_, err := s.DB.Exec("INSERT INTO audit(at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code,request_id,native_code,preview) VALUES(?,?,?,?,?,?,?,?,?,?,?)", a.At.UnixMilli(), a.AgentID, a.SourceID, a.Operation, a.Fingerprint, a.ElapsedMS, a.Rows, a.ErrorCode, a.RequestID, a.NativeCode, a.Preview)
	return err
}

func (s *Store) Audits(f AuditFilter, limit int) ([]model.Audit, error) {
	where, args := []string{"1=1"}, []any{}
	for _, v := range []struct {
		clause  string
		value   any
		enabled bool
	}{
		{"id<?", f.Before, f.Before > 0},
		{"agent_id=?", f.Agent, f.Agent != ""},
		{"source_id=?", f.Source, f.Source != ""},
		{"request_id=?", f.RequestID, f.RequestID != ""},
		{"at>=?", f.From.UnixMilli(), !f.From.IsZero()},
		{"at<=?", f.Until.UnixMilli(), !f.Until.IsZero()},
	} {
		if v.enabled {
			where = append(where, v.clause)
			args = append(args, v.value)
		}
	}
	if f.Status == "success" {
		where = append(where, "error_code=''")
	}
	if f.Status == "error" {
		where = append(where, "error_code<>''")
	}
	args = append(args, limit)
	rows, err := s.DB.Query("SELECT id,at,agent_id,source_id,operation,fingerprint,elapsed_ms,rows,error_code,request_id,native_code,preview FROM audit WHERE "+strings.Join(where, " AND ")+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	return scanAudits(rows)
}

// AuditBatch reads a bounded export page; the extra character lets the encoder
// report truncation without loading unbounded caller-supplied identifiers.
func (s *Store) AuditBatch(ctx context.Context, after int64) ([]model.Audit, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,at,substr(agent_id,1,2049),substr(source_id,1,2049),substr(operation,1,2049),substr(fingerprint,1,2049),elapsed_ms,rows,substr(error_code,1,2049),substr(request_id,1,2049),substr(native_code,1,2049),preview FROM audit WHERE id>? ORDER BY id LIMIT 64`, after)
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
		if err := rows.Scan(&a.ID, &at, &a.AgentID, &a.SourceID, &a.Operation, &a.Fingerprint, &a.ElapsedMS, &a.Rows, &a.ErrorCode, &a.RequestID, &a.NativeCode, &a.Preview); err != nil {
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
		(SELECT max(at) FROM audit WHERE agent_id=a.agent_id AND preview=0 AND error_code='')
		FROM audit a WHERE preview=0 AND id IN (SELECT max(id) FROM audit WHERE preview=0 GROUP BY agent_id)`)
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
