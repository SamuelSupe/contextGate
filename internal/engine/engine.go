package engine

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/SamuelSupe/mcpdbhub/internal/adapter"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"slices"
	"sync"
	"time"
)

type entry struct {
	conn     adapter.Connection
	err      error
	ready    chan struct{}
	revision int64
	refs     int
	retired  bool
}
type job struct {
	source, agent string
	cancel        context.CancelFunc
}
type Engine struct {
	Store           *store.Store
	FileRoot        string
	mu              sync.Mutex
	connections     map[string]*entry
	jobs            map[string]job
	active          int
	activeAgents    map[string]int
	activeSources   map[string]int
	capacityChanged chan struct{}
}

func New(s *store.Store) *Engine {
	return &Engine{
		Store: s, connections: map[string]*entry{}, jobs: map[string]job{},
		activeAgents: map[string]int{}, activeSources: map[string]int{},
		capacityChanged: make(chan struct{}),
	}
}
func (e *Engine) Authorize(p model.Principal, id string) (model.Source, error) {
	src, err := e.Store.Source(id)
	if err != nil || !src.Enabled {
		return src, model.Fail("not_found", "data source is unavailable")
	}
	if p.Admin {
		return src, nil
	}
	if p.CredentialValid != nil && !p.CredentialValid() {
		return src, model.Fail("unauthorized", "credential revoked or expired")
	}
	a, err := e.Store.Agent(p.AgentID)
	if err != nil || !a.Enabled || a.RevokedAt != nil || !a.ExpiresAt.After(time.Now()) || !slices.Contains(a.Sources, id) {
		return src, model.Fail("not_found", "data source is unavailable")
	}
	return src, nil
}
func (e *Engine) Sources(p model.Principal) ([]map[string]any, error) {
	sources, err := e.Store.Sources()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, s := range sources {
		if _, err := e.Authorize(p, s.ID); err != nil {
			continue
		}
		cap := adapter.ForSource(s)
		out = append(out, map[string]any{"id": s.ID, "name": s.Name, "kind": s.Kind, "version": s.Version, "database": s.Database, "capability": cap, "limits": s.Limits})
	}
	return out, nil
}
func (e *Engine) acquire(ctx context.Context, s model.Source) (*entry, error) {
	e.mu.Lock()
	en := e.connections[s.ID]
	if en != nil && en.revision == s.ExecutionRevision() {
		en.refs++
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			e.release(en)
			return nil, ctx.Err()
		case <-en.ready:
			if en.err != nil {
				e.release(en)
				return nil, en.err
			}
			return en, nil
		}
	}
	if en != nil {
		en.retired = true
		if en.refs == 0 && en.conn != nil {
			en.conn.Close()
		}
	}
	en = &entry{ready: make(chan struct{}), revision: s.ExecutionRevision(), refs: 1}
	e.connections[s.ID] = en
	e.mu.Unlock()
	var err error
	if e.FileRoot != "" && (s.Kind == "sqlite" || s.Kind == "duckdb") {
		err = adapter.ValidateSource(&s, e.FileRoot)
	}
	var conn adapter.Connection
	if err == nil {
		conn, err = adapter.Open(ctx, s)
	}
	e.mu.Lock()
	en.conn = conn
	en.err = err
	if err != nil && e.connections[s.ID] == en {
		delete(e.connections, s.ID)
		en.retired = true
	}
	close(en.ready)
	e.mu.Unlock()
	if err != nil {
		e.release(en)
		return nil, err
	}
	return en, nil
}
func (e *Engine) release(en *entry) {
	e.mu.Lock()
	en.refs--
	closeConn := en.refs == 0 && en.retired && en.conn != nil
	e.mu.Unlock()
	if closeConn {
		en.conn.Close()
	}
}
func (e *Engine) InvalidateSource(id string) {
	e.mu.Lock()
	en := e.connections[id]
	delete(e.connections, id)
	if en != nil {
		en.retired = true
	}
	for _, j := range e.jobs {
		if j.source == id {
			j.cancel()
		}
	}
	closeConn := en != nil && en.refs == 0 && en.conn != nil
	e.mu.Unlock()
	if closeConn {
		en.conn.Close()
	}
}
func (e *Engine) InvalidateAgent(id string) {
	e.mu.Lock()
	for _, j := range e.jobs {
		if j.agent == id {
			j.cancel()
		}
	}
	e.mu.Unlock()
}
func (e *Engine) Close() {
	e.mu.Lock()
	for _, j := range e.jobs {
		j.cancel()
	}
	ids := []string{}
	for id := range e.connections {
		ids = append(ids, id)
	}
	e.mu.Unlock()
	for _, id := range ids {
		e.InvalidateSource(id)
	}
}
func (e *Engine) fingerprint(q model.Query) string {
	q.Cursor = ""
	b, _ := json.Marshal(q)
	h := hmac.New(sha256.New, e.Store.Vault.Key)
	h.Write([]byte("query-fingerprint:"))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

type cursor struct {
	Agent       string `json:"agent"`
	Source      string `json:"source"`
	Operation   string `json:"operation"`
	Revision    int64  `json:"revision"`
	Fingerprint string `json:"fingerprint"`
	State       string `json:"state"`
	Expires     int64  `json:"expires"`
}

func (e *Engine) Execute(ctx context.Context, p model.Principal, operation string, q model.Query) (result *model.Result, err error) {
	started := time.Now()
	requestID := secure.Random(16)
	fp := e.fingerprint(q)
	principal := p.AgentID
	if p.Admin {
		principal = "admin"
	}
	defer func() {
		code, nativeCode := "", ""
		rows := 0
		if err != nil {
			safe := PublicError(err)
			safe.RequestID = requestID
			code, nativeCode, err = safe.Code, safe.NativeCode, safe
		}
		if result != nil {
			rows = result.RowCount
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.RequestID = requestID
		}
		if auditErr := e.Store.Audit(model.Audit{RequestID: requestID, NativeCode: nativeCode, Preview: p.Preview, At: started, AgentID: principal, SourceID: q.SourceID, Operation: operation, Fingerprint: fp, ElapsedMS: time.Since(started).Milliseconds(), Rows: rows, ErrorCode: code}); auditErr != nil {
			result = nil
			err = &model.Error{Code: "audit_unavailable", Message: "Query result withheld because audit storage is unavailable.", RequestID: requestID}
		}
	}()
	src, err := e.Authorize(p, q.SourceID)
	if err != nil {
		return nil, err
	}
	cap, _ := adapter.Get(src.Kind)
	metadata := operation == "namespaces" || operation == "objects" || operation == "describe"
	if !metadata && operation != cap.Tool {
		return nil, model.Fail("wrong_tool", "use "+cap.Tool+" for this data source")
	}
	l := src.Limits
	if q.MaxRows < 0 || q.TimeoutSeconds < 0 || q.MaxBytes < 0 {
		return nil, model.Fail("invalid_query", "query limits must be positive")
	}
	if q.MaxBytes > 0 {
		if q.MaxBytes > l.MaxBytes || q.MaxBytes < 1024 {
			return nil, model.Fail("limit_exceeded", "byte limit must be between 1024 and the data source limit")
		}
		l.MaxBytes = q.MaxBytes
	}
	if q.MaxRows > 0 {
		if q.MaxRows > l.MaxRows {
			return nil, model.Fail("limit_exceeded", "Agent cannot raise the configured row limit")
		}
		l.MaxRows = q.MaxRows
	}
	if q.TimeoutSeconds > 0 {
		if q.TimeoutSeconds > l.TimeoutSeconds {
			return nil, model.Fail("limit_exceeded", "Agent cannot raise the configured timeout")
		}
		l.TimeoutSeconds = q.TimeoutSeconds
	}
	if q.Cursor != "" {
		b, err := e.Store.Vault.Open(q.Cursor, "cursor")
		if err != nil {
			return nil, model.Fail("invalid_cursor", "cursor is invalid")
		}
		var c cursor
		if err = json.Unmarshal(b, &c); err != nil || c.Agent != principal || c.Source != src.ID || c.Operation != operation || c.Revision != src.ExecutionRevision() || c.Fingerprint != fp || c.Expires < time.Now().Unix() {
			return nil, model.Fail("invalid_cursor", "cursor expired or belongs to another query")
		}
		q.Cursor = c.State
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(l.TimeoutSeconds)*time.Second)
	defer cancel()
	defer func() {
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
		}
	}()
	id := secure.Random(12)
	e.mu.Lock()
	e.jobs[id] = job{src.ID, p.AgentID, cancel}
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.jobs, id); e.mu.Unlock() }()
	if fresh, check := e.Authorize(p, src.ID); check != nil || fresh.ExecutionRevision() != src.ExecutionRevision() {
		return nil, model.Fail("cancelled", "authorization or data source changed")
	}
	if err = e.admit(ctx, principal, src); err != nil {
		return nil, err
	}
	defer e.releaseAdmission(principal, src.ID)
	en, err := e.acquire(ctx, src)
	if err != nil {
		return nil, err
	}
	defer e.release(en)
	if metadata {
		if paged, ok := en.conn.(adapter.DiscoveryPager); ok {
			result, err = paged.DiscoverPage(ctx, operation, q, l)
			if err != nil {
				return nil, err
			}
		} else {
			objects, err := en.conn.Discover(ctx, operation, q.Namespace, q.Object)
			if err != nil {
				return nil, err
			}
			result = model.NewResult("metadata")
			for _, o := range objects {
				ok, err := result.Add(o, l)
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
			}
		}
	} else {
		result, err = en.conn.Query(ctx, q, l)
		if err != nil {
			return nil, err
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if fresh, check := e.Authorize(p, src.ID); check != nil || fresh.ExecutionRevision() != src.ExecutionRevision() {
		return nil, model.Fail("cancelled", "authorization or data source changed")
	}
	if result.NextCursor != "" {
		b, _ := json.Marshal(cursor{principal, src.ID, operation, src.ExecutionRevision(), fp, result.NextCursor, time.Now().Add(5 * time.Minute).Unix()})
		result.NextCursor = e.Store.Vault.Seal(b, "cursor")
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	textJSON, _ := json.Marshal(string(b))
	if len(b)+len(textJSON)+512 > l.MaxBytes {
		return nil, model.Fail("result_too_large", "encoded result exceeds byte limit")
	}
	return result, nil
}
func (e *Engine) Probe(ctx context.Context, s model.Source) (model.Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := adapter.Open(ctx, s)
	if err != nil {
		return model.Probe{}, err
	}
	defer c.Close()
	p, err := c.Probe(ctx)
	if err != nil {
		return p, err
	}
	return p, nil
}
