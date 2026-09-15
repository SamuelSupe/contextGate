package engine

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/ontology"
	"github.com/SamuelSupe/contextGate/internal/semantic"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type controlledConnection struct {
	started chan struct{}
	release chan struct{}
	version atomic.Int32
}

func (c *controlledConnection) Close() error { return nil }
func (c *controlledConnection) Probe(context.Context) (model.Probe, error) {
	return model.Probe{ServerVersion: fmt.Sprintf("fixture-v%d", 1+c.version.Load())}, nil
}
func (c *controlledConnection) Discover(context.Context, string, string, string) ([]model.Object, error) {
	return nil, nil
}
func (c *controlledConnection) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if q.Query == "block" {
		c.started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if q.Query == "hold" {
		c.started <- struct{}{}
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
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
	st, e := store.Open(t.TempDir(), testpg.DSN(t, t.TempDir()))
	if e != nil {
		t.Fatal(e)
	}
	en := New(st)
	t.Cleanup(func() { en.Close(); st.Close() })
	src := model.Source{ObservedVersion: "fixture-v1", ID: "source", Name: "fixture", Kind: "sqlite", Enabled: true, Revision: 1}
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
	c := &controlledConnection{started: make(chan struct{}, 8)}
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
	// Two administrators may preview as the same query Agent; their pagination
	// still belongs to the actual operator and the credential used to start it.
	p = model.Principal{AgentID: "a", Preview: true, AdministratorID: "owner", AdministratorRole: model.RoleAdministrator, SessionIdentity: "session-one", CredentialVersion: 1}
	q = model.Query{SourceID: src.ID, Query: "page"}
	r, e = en.Execute(context.Background(), p, "query_sql", q)
	if e != nil {
		t.Fatal(e)
	}
	q.Cursor = r.NextCursor
	for _, change := range []func(*model.Principal){
		func(v *model.Principal) { v.AdministratorID = "colleague" },
		func(v *model.Principal) { v.SessionIdentity = "session-two" },
		func(v *model.Principal) { v.CredentialVersion++ },
		func(v *model.Principal) { v.ConfigurationAgentID = "configuration-identity" },
	} {
		other := p
		change(&other)
		if _, e = en.Execute(context.Background(), other, "query_sql", q); model.ErrorCode(e) != "invalid_cursor" {
			t.Fatalf("preview cursor crossed administrator credentials: %v", e)
		}
	}
	if _, e = en.Execute(context.Background(), p, "query_sql", q); e != nil {
		t.Fatalf("original preview cursor rejected: %v", e)
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

func TestTemplatePublicationCancellationAndCursorBinding(t *testing.T) {
	en, c, src := testEngine(t)
	snapshot := semantic.Empty()
	for _, name := range []string{"block", "page"} {
		template := semantic.Template{Enabled: true, Tool: "query_sql", QueryJSON: `{"query":"` + name + `"}`, Parameters: []semantic.Parameter{}, ExampleJSON: `{}`}
		snapshot.Entries = append(snapshot.Entries, semantic.Entry{ID: name, Kind: "template", Name: name, Template: &template})
		if err := en.Store.SaveSemanticEvidence(src.ID, semantic.Evidence{Definition: semantic.Definition(template), Connection: en.connectionProof(src), CheckedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	st := semantic.State{Draft: snapshot, Published: semantic.Empty()}
	if err := en.Store.WriteSemantics(src.ID, 0, st); err != nil {
		t.Fatal(err)
	}
	st, err := en.PublishSemantics(context.Background(), src.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	p := model.Principal{AgentID: "a"}
	page := semantic.Execution{SourceID: src.ID, TemplateID: "page", ExecutionVersion: "1", Parameters: map[string]any{}}
	result, err := en.ExecuteTemplate(context.Background(), p, page)
	if err != nil {
		t.Fatal(err)
	}
	page.Cursor = result.NextCursor
	if _, err = en.ExecuteTemplate(context.Background(), model.Principal{AgentID: "b"}, page); model.ErrorCode(err) != "invalid_cursor" {
		t.Fatal("cursor crossed Agent identity", err)
	}
	if _, err = en.Execute(context.Background(), p, "query_sql", model.Query{SourceID: src.ID, Query: "page", Cursor: page.Cursor}); model.ErrorCode(err) != "invalid_cursor" {
		t.Fatal("template cursor used by raw query", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := en.ExecuteTemplate(context.Background(), p, semantic.Execution{SourceID: src.ID, TemplateID: "block", ExecutionVersion: "1", Parameters: map[string]any{}})
		done <- err
	}()
	select {
	case <-c.started:
	case <-time.After(time.Second):
		t.Fatal("template did not begin")
	}
	en.InvalidateTemplates(src.ID, map[string]bool{"block": true}, map[string]string{"block": "1"})
	st.Draft.Overview = "Documentation-only publication"
	if err = en.Store.WriteSemantics(src.ID, st.Revision, st); err != nil {
		t.Fatal(err)
	}
	st, err = en.PublishSemantics(context.Background(), src.ID, st.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatal("description publication cancelled query", err)
	case <-time.After(40 * time.Millisecond):
	}
	for i := range st.Draft.Entries {
		if st.Draft.Entries[i].ID == "block" {
			st.Draft.Entries[i].Template.Enabled = false
		}
	}
	if err = en.Store.WriteSemantics(src.ID, st.Revision, st); err != nil {
		t.Fatal(err)
	}
	st, err = en.PublishSemantics(context.Background(), src.ID, st.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if model.ErrorCode(err) != "cancelled" {
			t.Fatal("disabled template returned data", err)
		}
	case <-time.After(time.Second):
		t.Fatal("template disable did not cancel")
	}
	if _, err = en.ExecuteTemplate(context.Background(), p, page); err != nil {
		t.Fatal("unaffected template cursor invalidated", err)
	}
	for i := range st.Draft.Entries {
		if st.Draft.Entries[i].ID == "page" {
			st.Draft.Entries[i].Template.QueryJSON = `{"query":"changed"}`
			en.Store.SaveSemanticEvidence(src.ID, semantic.Evidence{Definition: semantic.Definition(*st.Draft.Entries[i].Template), Connection: en.connectionProof(src), CheckedAt: time.Now()})
		}
	}
	if err = en.Store.WriteSemantics(src.ID, st.Revision, st); err != nil {
		t.Fatal(err)
	}
	st, err = en.PublishSemantics(context.Background(), src.ID, st.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = en.ExecuteTemplate(context.Background(), p, page); model.ErrorCode(err) != "template_changed" {
		t.Fatal("obsolete template executed", err)
	}
	page.Cursor = ""
	page.ExecutionVersion = fmt.Sprint(st.PublishedVersion)
	if _, err = en.ExecuteTemplate(context.Background(), p, page); err != nil {
		t.Fatal("new execution version failed", err)
	}
	c.version.Store(1)
	if _, err = en.ExecuteTemplate(context.Background(), p, page); err == nil {
		t.Fatal("database upgrade did not pause template")
	}
	updated, err := en.Store.Source(src.ID)
	if err != nil || updated.ObservedVersion != "fixture-v2" || updated.ConnectionRevision <= src.ConnectionRevision {
		t.Fatal("database version evidence was not invalidated", err)
	}
	if _, err = en.PublishSemantics(context.Background(), src.ID, st.Revision); err == nil {
		t.Fatal("stale database proof republished")
	}
}

func TestOntologyPublicationKeepsExecutionStartContext(t *testing.T) {
	en, c, src := testEngine(t)
	c.release = make(chan struct{})
	d := ontology.Empty()
	d.Name = "Commerce"
	d.Entities = []ontology.Entity{{ID: "customer", Name: "Customer"}, {ID: "order", Name: "Order"}}
	owner := ontology.State{ID: "commerce", Draft: d}
	if err := en.Store.WriteOntology(owner, 0, true); err != nil {
		t.Fatal(err)
	}
	owner, _ = en.Store.Ontology(owner.ID)
	owner.Draft.Description = "New definition"
	if err := en.Store.WriteOntology(owner, owner.Revision, true); err != nil {
		t.Fatal(err)
	}
	b := &ontology.Binding{OntologyID: owner.ID, Version: 1, Entities: []ontology.EntityMapping{{Entity: "customer", Objects: []ontology.Reference{{Object: "customers"}}}, {Entity: "order", Objects: []ontology.Reference{{Object: "orders"}}}}}
	template := semantic.Template{Enabled: true, Tool: "query_sql", QueryJSON: `{"query":"hold"}`, ExampleJSON: `{}`, Parameters: []semantic.Parameter{}, ConceptRefs: []string{ontology.Ref("entity_type", "customer")}}
	for _, key := range []string{semantic.Definition(template), mappingProof(b)} {
		if err := en.Store.SaveSemanticEvidence(src.ID, semantic.Evidence{Definition: key, Connection: en.connectionProof(src), CheckedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	draft := semantic.Empty()
	draft.Ontology = b
	draft.Entries = []semantic.Entry{{ID: "held", Kind: "template", Name: "Held query", Template: &template}}
	pageTemplate := template
	pageTemplate.QueryJSON = `{"query":"page"}`
	draft.Entries = append(draft.Entries, semantic.Entry{ID: "paged", Kind: "template", Name: "Paged query", Template: &pageTemplate})
	if err := en.Store.SaveSemanticEvidence(src.ID, semantic.Evidence{Definition: semantic.Definition(pageTemplate), Connection: en.connectionProof(src), CheckedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := en.Store.WriteSemantics(src.ID, 0, semantic.State{Draft: draft, Published: semantic.Empty()}); err != nil {
		t.Fatal(err)
	}
	st, err := en.PublishSemantics(context.Background(), src.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	pageInput := semantic.Execution{SourceID: src.ID, TemplateID: "paged", ExecutionVersion: "1", Parameters: map[string]any{}}
	first, err := en.ExecuteTemplate(context.Background(), model.Principal{AgentID: "a"}, pageInput)
	if err != nil {
		t.Fatal(err)
	}
	pageInput.Cursor = first.NextCursor
	done := make(chan *model.Result, 1)
	failed := make(chan error, 1)
	go func() {
		result, err := en.ExecuteTemplate(context.Background(), model.Principal{AgentID: "a"}, semantic.Execution{SourceID: src.ID, TemplateID: "held", ExecutionVersion: "1", Parameters: map[string]any{}})
		if err != nil {
			failed <- err
		} else {
			done <- result
		}
	}()
	select {
	case <-c.started:
	case <-time.After(time.Second):
		t.Fatal("query did not start")
	}
	st.Draft.Ontology.Version = 2
	st.Draft.Entries[0].Template.ConceptRefs = []string{ontology.Ref("entity_type", "order")}
	if err = en.Store.SaveSemanticEvidence(src.ID, semantic.Evidence{Definition: mappingProof(st.Draft.Ontology), Connection: en.connectionProof(src), CheckedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err = en.Store.WriteSemantics(src.ID, st.Revision, st); err != nil {
		t.Fatal(err)
	}
	if _, err = en.PublishSemantics(context.Background(), src.ID, st.Revision+1); err != nil {
		t.Fatal(err)
	}
	close(c.release)
	defer func() {
		if _, err := en.ExecuteTemplate(context.Background(), model.Principal{AgentID: "a"}, pageInput); model.ErrorCode(err) != "invalid_cursor" {
			t.Fatal("ontology adoption accepted an old native cursor", err)
		}
	}()
	select {
	case result := <-done:
		if result.OntologyContext == nil || result.OntologyContext.Version != "1" || result.OntologyContext.ConceptRefs[0] != ontology.Ref("entity_type", "customer") {
			t.Fatal("request-start ontology context changed", result)
		}
	case err := <-failed:
		t.Fatal("definition publication cancelled execution", err)
	case <-time.After(time.Second):
		t.Fatal("query did not return")
	}
}
