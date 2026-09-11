package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j/config"
	"math"
	"strings"
	"time"
)

type neoConn struct {
	driver neo4j.Driver
	s      model.Source
}

func openNeo4j(ctx context.Context, s model.Source) (Connection, error) {
	tc, err := tlsConfig(s)
	if err != nil {
		return nil, err
	}
	scheme := "bolt"
	if s.TLSMode != "disable" {
		scheme = "bolt+s"
	}
	d, e := neo4j.NewDriver(scheme+"://"+address(s), neo4j.BasicAuth(s.Username, s.Password, ""), func(cfg *config.Config) {
		cfg.MaxConnectionPoolSize = s.Limits.Concurrency
		cfg.TlsConfig = tc
		cfg.ConnectionAcquisitionTimeout = 10 * time.Second
	})
	if e != nil {
		return nil, e
	}
	if e = d.VerifyConnectivity(ctx); e != nil {
		d.Close(ctx)
		return nil, e
	}
	return &neoConn{d, s}, nil
}
func (c *neoConn) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.driver.Close(ctx)
}
func (c *neoConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Connected: true, Protection: "engine_classification", PermissionStatus: "unverified", Evidence: []string{"EXPLAIN must report read-only; write clauses, procedures and external loading are rejected; driver read routing is not a permission guarantee"}, CheckedAt: time.Now()}
	info, e := c.driver.GetServerInfo(ctx)
	if e == nil {
		p.ServerVersion = info.Agent()
	}
	return p, e
}
func guardCypher(q string) error {
	ts, e := lex(q)
	if e != nil {
		return model.Fail("query_denied", e.Error())
	}
	bad := wordset("create merge set delete detach remove drop alter grant deny revoke call load foreach start use insert finish transaction transactions terminate")
	structural := wordset("match where return with and or not in all any none single reduce exists count collect as")
	for i, t := range ts {
		if !t.quoted && bad[t.text] {
			return model.Fail("query_denied", "Cypher writes, procedures and external loading are denied")
		}
		if i+1 < len(ts) && ts[i+1].text == "(" && len(t.text) > 0 && (t.text[0] >= 'a' && t.text[0] <= 'z') {
			if (i > 0 && ts[i-1].text == ".") || (!structural[t.text] && !pureFunctions[t.text]) {
				return model.Fail("query_denied", "custom Cypher functions are denied")
			}
		}
	}
	return nil
}
func graphValue(v any) any {
	switch n := v.(type) {
	case neo4j.Node:
		return map[string]any{"kind": "node", "element_id": n.ElementId, "labels": n.Labels, "properties": value(n.Props)}
	case neo4j.Relationship:
		return map[string]any{"kind": "relationship", "element_id": n.ElementId, "start_element_id": n.StartElementId, "end_element_id": n.EndElementId, "type": n.Type, "properties": value(n.Props)}
	case neo4j.Path:
		nodes := []any{}
		edges := []any{}
		for _, x := range n.Nodes {
			nodes = append(nodes, graphValue(x))
		}
		for _, x := range n.Relationships {
			edges = append(edges, graphValue(x))
		}
		return map[string]any{"kind": "path", "nodes": nodes, "relationships": edges}
	case []any:
		out := make([]any, len(n))
		for i, v := range n {
			out[i] = graphValue(v)
		}
		return out
	}
	return value(v)
}
func (c *neoConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if e := guardCypher(q.Query); e != nil {
		return nil, e
	}
	return c.run(ctx, q.Query, q.NamedParams, l, true)
}
func (c *neoConn) run(ctx context.Context, query string, p map[string]any, l model.Limits, classify bool) (*model.Result, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.s.Database, AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)
	tx, e := session.BeginTransaction(ctx, neo4j.WithTxTimeout(time.Duration(l.TimeoutSeconds)*time.Second))
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	args := map[string]any{}
	for k, v := range p {
		args[k], e = cypherParam(v)
		if e != nil {
			return nil, e
		}
	}
	if classify {
		ex, e := tx.Run(ctx, "EXPLAIN "+query, args)
		if e != nil {
			return nil, e
		}
		sum, e := ex.Consume(ctx)
		if e != nil {
			return nil, e
		}
		if sum.QueryType() != neo4j.QueryTypeReadOnly {
			return nil, model.Fail("query_denied", "Neo4j did not classify this query as read-only")
		}
	}
	rows, e := tx.Run(ctx, query, args)
	if e != nil {
		return nil, e
	}
	keys, e := rows.Keys()
	if e != nil {
		return nil, e
	}
	r := model.NewResult("graph")
	for _, key := range keys {
		r.Columns = append(r.Columns, model.Column{Name: key, Type: "cypher"})
	}
	for rows.Next(ctx) {
		rec := rows.Record()
		v := make([]any, len(rec.Values))
		for i, item := range rec.Values {
			v[i] = graphValue(item)
		}
		ok, e := r.Add(v, l)
		if e != nil {
			return nil, e
		}
		if !ok {
			break
		}
	}
	return r, rows.Err()
}

func cypherParam(v any) (any, error) {
	switch n := v.(type) {
	case json.Number:
		if integer, err := n.Int64(); err == nil {
			return integer, nil
		}
		if !strings.ContainsAny(string(n), ".eE") {
			return nil, model.Fail("invalid_query", "Cypher integer parameter exceeds int64")
		}
		number, err := n.Float64()
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, model.Fail("invalid_query", "Cypher floating parameter exceeds float64")
		}
		return number, nil
	case []any:
		out := make([]any, len(n))
		for i, item := range n {
			var err error
			out[i], err = cypherParam(item)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(n))
		for key, item := range n {
			var err error
			out[key], err = cypherParam(item)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	default:
		return v, nil
	}
}
func (c *neoConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if op == "namespaces" {
		return []model.Object{{Name: c.s.Database, Type: "database"}}, nil
	}
	if ns != "" && ns != c.s.Database {
		return nil, errors.New("namespace outside configured database")
	}
	query := "CALL db.labels() YIELD label RETURN label"
	p := map[string]any{}
	if op == "describe" {
		query = "MATCH (n) WHERE $label IN labels(n) UNWIND keys(n) AS property RETURN DISTINCT property LIMIT 1000"
		p["label"] = obj
	}
	r, e := c.run(ctx, query, p, c.s.Limits, false)
	if e != nil {
		return nil, e
	}
	out := []model.Object{}
	for _, row := range r.Data {
		vals, ok := row.([]any)
		if ok && len(vals) > 0 {
			typ := "label"
			if op == "describe" {
				typ = "property"
			}
			out = append(out, model.Object{Name: strings.TrimSpace(model.String(vals[0])), Namespace: c.s.Database, Type: typ})
		}
	}
	return out, nil
}
