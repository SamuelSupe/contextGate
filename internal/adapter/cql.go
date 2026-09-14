package adapter

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/gocql/gocql"
	"strings"
	"time"
)

type cqlConn struct {
	session *gocql.Session
	s       model.Source
}

func openCQL(ctx context.Context, s model.Source) (Connection, error) {
	cfg := gocql.NewCluster(strings.Split(s.Host, ",")...)
	cfg.Port = s.Port
	cfg.Keyspace = s.Database
	cfg.Timeout = time.Duration(s.Limits.TimeoutSeconds) * time.Second
	cfg.ConnectTimeout = 10 * time.Second
	cfg.NumConns = 1
	cfg.Consistency = gocql.LocalOne
	cfg.DisableInitialHostLookup = true
	if s.Username != "" {
		cfg.Authenticator = gocql.PasswordAuthenticator{Username: s.Username, Password: s.Password}
	}
	tc, e := tlsConfig(s)
	if e != nil {
		return nil, e
	}
	if tc != nil {
		cfg.SslOpts = &gocql.SslOptions{Config: tc, EnableHostVerification: true}
	}
	session, e := cfg.CreateSession()
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		session.Close()
		return nil, e
	}
	return &cqlConn{session, s}, nil
}
func (c *cqlConn) Close() error { c.session.Close(); return nil }
func (c *cqlConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Connected: true, Protection: "select_grammar_and_database_permissions", PermissionStatus: "unverified", Evidence: []string{"Only a single CQL SELECT is exposed; provision a SELECT-only account externally"}, CheckedAt: time.Now()}
	query := "SELECT release_version FROM system.local"
	if c.s.Kind == "scylladb" {
		query = "SELECT version FROM system.versions WHERE key='local'"
	}
	e := c.session.Query(query).WithContext(ctx).Scan(&p.ServerVersion)
	return p, e
}
func (c *cqlConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if e := guardSQL(c.s.Kind, q.Query); e != nil {
		return nil, e
	}
	var state []byte
	var e error
	if q.Cursor != "" {
		state, e = base64.RawURLEncoding.DecodeString(q.Cursor)
		if e != nil {
			return nil, errors.New("invalid cursor")
		}
	}
	args := make([]any, len(q.Params))
	for i, v := range q.Params {
		args[i] = cqlParam{v}
	}
	iter := c.session.Query(q.Query, args...).WithContext(ctx).PageSize(l.MaxRows).PageState(state).Iter()
	r := model.NewResult("table")
	cols := iter.Columns()
	for _, col := range cols {
		r.Columns = append(r.Columns, model.Column{Name: col.Name, Type: fmt.Sprint(col.TypeInfo)})
	}
	for {
		row := map[string]any{}
		if !iter.MapScan(row) {
			break
		}
		v := make([]any, len(cols))
		for i, col := range cols {
			v[i] = value(row[col.Name])
		}
		ok, e := r.Add(v, l)
		if e != nil {
			iter.Close()
			return nil, e
		}
		if !ok {
			break
		}
	}
	if !r.Truncated && len(iter.PageState()) > 0 {
		r.NextCursor = base64.RawURLEncoding.EncodeToString(iter.PageState())
	}
	e = iter.Close()
	return r, e
}
func (c *cqlConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if ns == "" {
		ns = c.s.Database
	}
	out := []model.Object{}
	if op == "namespaces" {
		return []model.Object{{Name: c.s.Database, Type: "keyspace"}}, nil
	}
	if ns != c.s.Database {
		return nil, model.Fail("query_denied", "namespace is outside configured keyspace")
	}
	if op == "describe" {
		iter := c.session.Query("SELECT column_name,type FROM system_schema.columns WHERE keyspace_name=? AND table_name=?", ns, obj).WithContext(ctx).Iter()
		var name, typ string
		for iter.Scan(&name, &typ) {
			out = append(out, model.Object{Name: name, Type: typ, Namespace: ns})
		}
		return out, iter.Close()
	}
	iter := c.session.Query("SELECT table_name FROM system_schema.tables WHERE keyspace_name=?", ns).WithContext(ctx).Iter()
	var name string
	for iter.Scan(&name) {
		out = append(out, model.Object{Name: name, Type: "table", Namespace: ns})
		if len(out) >= 1000 {
			break
		}
	}
	return out, iter.Close()
}
