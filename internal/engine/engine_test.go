package engine

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type controlledConnection struct{ started chan struct{} }

func (c *controlledConnection) Close() error                               { return nil }
func (c *controlledConnection) Probe(context.Context) (model.Probe, error) { return model.Probe{}, nil }
func (c *controlledConnection) Discover(context.Context, string, string, string) ([]model.Object, error) {
	return nil, nil
}
func (c *controlledConnection) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if q.Query == "block" {
		c.started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	r := model.NewResult("values")
	r.Add("row", l)
	if q.Query == "page" {
		r.NextCursor = "native-state"
	}
	return r, nil
}
func testEngine(t *testing.T) (*Engine, *controlledConnection, model.Source) {
	t.Helper()
	st, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	en := New(st)
	t.Cleanup(func() { en.Close(); st.Close() })
	src := model.Source{ID: "source", Name: "fixture", Kind: "sqlite", Enabled: true, Revision: 1}
	src.Limits.Defaults()
	src.Limits.Concurrency = 1
	if e = st.SaveSource(src); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"a", "b"} {
		if e = st.SaveAgent(model.Agent{ID: id, Enabled: true, ExpiresAt: time.Now().Add(time.Hour), Sources: []string{src.ID}}, ""); e != nil {
			t.Fatal(e)
		}
	}
	c := &controlledConnection{make(chan struct{}, 8)}
	ready := make(chan struct{})
	close(ready)
	en.connections[src.ID] = &entry{conn: c, ready: ready, revision: src.Revision}
	return en, c, src
}
func TestRevocationCancelsRunningAndQueuedWork(t *testing.T) {
	en, c, src := testEngine(t)
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			_, e := en.Execute(context.Background(), model.Principal{AgentID: "a"}, "query_sql", model.Query{SourceID: src.ID, Query: "block"})
			done <- e
		}()
	}
	select {
	case <-c.started:
	case <-time.After(time.Second):
		t.Fatal("query did not start")
	}
	select {
	case <-c.started:
		t.Fatal("source concurrency exceeded")
	case <-time.After(40 * time.Millisecond):
	}
	a, _ := en.Store.Agent("a")
	a.Enabled = false
	en.Store.SaveAgent(a, "")
	en.InvalidateAgent("a")
	for i := 0; i < 4; i++ {
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("revoked work returned data")
			}
		case <-time.After(time.Second):
			t.Fatal("revoked work was not cancelled")
		}
	}
	if _, e := en.Execute(context.Background(), model.Principal{AgentID: "b"}, "query_sql", model.Query{SourceID: src.ID, Query: "next"}); e != nil {
		t.Fatalf("slot or connection leaked: %v", e)
	}
}
func TestCursorBindingAndCredentialRecheck(t *testing.T) {
	en, _, src := testEngine(t)
	p := model.Principal{AgentID: "a"}
	q := model.Query{SourceID: src.ID, Query: "page"}
	r, e := en.Execute(context.Background(), p, "query_sql", q)
	if e != nil {
		t.Fatal(e)
	}
	q.Cursor = r.NextCursor
	if _, e = en.Execute(context.Background(), p, "query_sql", q); e != nil {
		t.Fatal(e)
	}
	if _, e = en.Execute(context.Background(), p, "objects", q); model.ErrorCode(e) != "invalid_cursor" {
		t.Fatalf("cross-operation cursor: %v", e)
	}
	if _, e = en.Execute(context.Background(), model.Principal{AgentID: "b"}, "query_sql", q); model.ErrorCode(e) != "invalid_cursor" {
		t.Fatalf("cross-agent cursor: %v", e)
	}
	q.Query = "other"
	if _, e = en.Execute(context.Background(), p, "query_sql", q); model.ErrorCode(e) != "invalid_cursor" {
		t.Fatalf("cross-query cursor: %v", e)
	}
	q.Query = "page"
	q.Cursor += "tampered"
	if _, e = en.Execute(context.Background(), p, "query_sql", q); model.ErrorCode(e) != "invalid_cursor" {
		t.Fatalf("tampered cursor: %v", e)
	}
	var valid atomic.Bool
	valid.Store(true)
	p.CredentialValid = valid.Load
	valid.Store(false)
	if _, e = en.Execute(context.Background(), p, "query_sql", model.Query{SourceID: src.ID, Query: "next"}); model.ErrorCode(e) != "unauthorized" {
		t.Fatalf("revoked OAuth credential snapshot accepted: %v", e)
	}
}

func TestAdmissionLimitsAndQueueIsolation(t *testing.T) {
	for _, scope := range []string{"agent", "source", "global"} {
		t.Run(scope, func(t *testing.T) {
			en, c, template := testEngine(t)
			c.started = make(chan struct{}, 64)
			ids := make([]string, 40)
			for i := range ids {
				src := template
				src.ID = fmt.Sprintf("source-%d", i)
				ids[i] = src.ID
				if err := en.Store.SaveSource(src); err != nil {
					t.Fatal(err)
				}
				ready := make(chan struct{})
				close(ready)
				en.connections[src.ID] = &entry{conn: c, ready: ready, revision: src.Revision}
			}
			for i := range ids {
				if err := en.Store.SaveAgent(model.Agent{ID: fmt.Sprintf("agent-%d", i), Enabled: true, Sources: ids, ExpiresAt: time.Now().Add(time.Hour)}, ""); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 40)
			for i := range ids {
				agent, source := fmt.Sprintf("agent-%d", i), ids[i]
				if scope == "agent" {
					agent = "agent-0"
				}
				if scope == "source" {
					source = ids[0]
				}
				go func() {
					_, err := en.Execute(ctx, model.Principal{AgentID: agent}, "query_sql", model.Query{SourceID: source, Query: "block"})
					done <- err
				}()
			}
			limit := map[string]int{"agent": 4, "source": 1, "global": 32}[scope]
			for i := 0; i < limit; i++ {
				select {
				case <-c.started:
				case <-time.After(3 * time.Second):
					t.Fatal("queries did not start")
				}
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				en.mu.Lock()
				queued := len(en.jobs) == 40
				en.mu.Unlock()
				if queued {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("requests did not enter the queue")
				}
				time.Sleep(time.Millisecond)
			}
			select {
			case <-c.started:
				t.Fatal("execution limit exceeded")
			case <-time.After(40 * time.Millisecond):
			}
			fastCtx, stop := context.WithTimeout(context.Background(), 200*time.Millisecond)
			_, err := en.Execute(fastCtx, model.Principal{AgentID: "b"}, "query_sql", model.Query{SourceID: template.ID, Query: "next"})
			stop()
			if scope == "global" {
				if model.ErrorCode(err) != "timeout" {
					t.Fatalf("global limit not enforced: %v", err)
				}
			} else if err != nil {
				t.Fatalf("queued %s work blocked unrelated Agent/source: %v", scope, err)
			}
			cancel()
			for range ids {
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("cancelled query succeeded")
					}
				case <-time.After(3 * time.Second):
					t.Fatal("queue was not cancelled")
				}
			}
			if _, err := en.Execute(context.Background(), model.Principal{AgentID: "b"}, "query_sql", model.Query{SourceID: template.ID, Query: "next"}); err != nil {
				t.Fatal(err)
			}
			en.mu.Lock()
			defer en.mu.Unlock()
			if en.active != 0 || len(en.activeAgents) != 0 || len(en.activeSources) != 0 {
				t.Fatal("admission reservation leaked")
			}
		})
	}
}
func TestDeadlineReleasesExecutionSlot(t *testing.T) {
	en, c, src := testEngine(t)
	p := model.Principal{AgentID: "a"}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, e := en.Execute(ctx, p, "query_sql", model.Query{SourceID: src.ID, Query: "block"}); model.ErrorCode(e) != "timeout" {
		t.Fatalf("deadline: %v", e)
	}
	<-c.started
	if _, e := en.Execute(context.Background(), p, "query_sql", model.Query{SourceID: src.ID, Query: "next"}); e != nil {
		t.Fatal(e)
	}
}

func TestDiagnosticsDoNotExposeDriverMessages(t *testing.T) {
	secret := "credential-and-query-must-not-leak"
	for _, tc := range []struct {
		err          error
		code, native string
	}{
		{&pgconn.PgError{Code: "28P01", Message: secret}, "database_authentication", "28P01"},
		{&pgconn.PgError{Code: "42501", Message: secret}, "database_permission", "42501"},
		{&mysql.MySQLError{Number: 1064, Message: secret}, "database_query", "1064"},
		{&net.DNSError{Err: secret}, "database_dns", ""},
		{x509.UnknownAuthorityError{}, "database_tls", ""},
		{errors.New(secret), "database_error", ""},
	} {
		got := PublicError(tc.err)
		if got.Code != tc.code || got.NativeCode != tc.native || strings.Contains(got.Message, secret) {
			t.Fatalf("unsafe or incorrect diagnostic: %+v", got)
		}
	}
}
