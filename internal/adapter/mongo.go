package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"strings"
	"sync"
	"time"
)

type mongoConn struct {
	client  *mongo.Client
	s       model.Source
	mu      sync.Mutex
	cursors map[string]*mongoPage
}

func openMongo(ctx context.Context, s model.Source) (Connection, error) {
	tc, e := tlsConfig(s)
	if e != nil {
		return nil, e
	}
	hosts := strings.Split(s.Host, ",")
	for i, h := range hosts {
		copy := s
		copy.Host = h
		hosts[i] = address(copy)
	}
	o := options.Client().SetHosts(hosts).SetConnectTimeout(10 * time.Second).SetServerSelectionTimeout(10 * time.Second).SetMaxPoolSize(uint64(s.Limits.Concurrency))
	if tc != nil {
		o.SetTLSConfig(tc)
	}
	if s.Username != "" {
		authSource := s.Options["auth_source"]
		if authSource == "" {
			authSource = s.Database
		}
		o.SetAuth(options.Credential{Username: s.Username, Password: s.Password, AuthSource: authSource})
	}
	if v := s.Options["replica_set"]; v != "" {
		o.SetReplicaSet(v)
	}
	client, e := mongo.Connect(o)
	if e != nil {
		return nil, e
	}
	if e = client.Ping(ctx, readpref.PrimaryPreferred()); e != nil {
		client.Disconnect(ctx)
		return nil, e
	}
	return &mongoConn{client: client, s: s, cursors: map[string]*mongoPage{}}, nil
}
func (c *mongoConn) Close() error {
	c.mu.Lock()
	for id, p := range c.cursors {
		p.timer.Stop()
		closeMongoCursor(p.cursor)
		delete(c.cursors, id)
	}
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.client.Disconnect(ctx)
}
func (c *mongoConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Connected: true, Protection: "read_role_and_operation_allowlist", PermissionStatus: "unverified", Evidence: []string{"Only find, aggregate without writes, count and distinct are exposed"}, CheckedAt: time.Now()}
	var build bson.M
	if e := c.client.Database(c.s.Database).RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&build); e == nil {
		p.ServerVersion = model.String(build["version"])
	}
	var status struct {
		AuthInfo struct {
			AuthenticatedUserPrivileges []struct {
				Actions []string `bson:"actions"`
			} `bson:"authenticatedUserPrivileges"`
		} `bson:"authInfo"`
	}
	if e := c.client.Database(c.s.Database).RunCommand(ctx, bson.D{{Key: "connectionStatus", Value: 1}, {Key: "showPrivileges", Value: true}}).Decode(&status); e == nil && len(status.AuthInfo.AuthenticatedUserPrivileges) > 0 {
		allowed := wordset("find listcollections listindexes dbstats collstats indexstats killcursors changestream")
		safe := true
		for _, priv := range status.AuthInfo.AuthenticatedUserPrivileges {
			for _, a := range priv.Actions {
				if !allowed[strings.ToLower(a)] {
					safe = false
				}
			}
		}
		if safe {
			p.PermissionStatus = "verified"
			p.Evidence = append(p.Evidence, "Effective actions contain only database read/metadata privileges")
		}
	}
	return p, nil
}
func mongoDocument(raw json.RawMessage) (bson.D, error) {
	if len(raw) == 0 {
		return bson.D{}, nil
	}
	var d bson.D
	e := bson.UnmarshalExtJSON(raw, false, &d)
	if e != nil {
		return nil, model.Fail("invalid_query", "invalid BSON document")
	}
	return d, guardMongo(d)
}
func guardMongo(v any) error {
	switch n := v.(type) {
	case bson.D:
		for _, f := range n {
			if wordset("$out $merge $where $function $accumulator $eval")[strings.ToLower(f.Key)] {
				return model.Fail("query_denied", "MongoDB write stages and JavaScript are denied")
			}
			if e := guardMongo(f.Value); e != nil {
				return e
			}
		}
	case bson.A:
		for _, x := range n {
			if e := guardMongo(x); e != nil {
				return e
			}
		}
	case []bson.D:
		for _, x := range n {
			if e := guardMongo(x); e != nil {
				return e
			}
		}
	}
	return nil
}
func mongoValue(v any) (any, error) {
	b, e := bson.MarshalExtJSON(bson.D{{Key: "value", Value: v}}, true, false)
	if e != nil {
		return nil, e
	}
	var out map[string]any
	e = json.Unmarshal(b, &out)
	return out["value"], e
}
func (c *mongoConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if q.Object == "" || strings.ContainsRune(q.Object, '\x00') {
		return nil, errors.New("collection is required")
	}
	if q.Namespace != "" && q.Namespace != c.s.Database {
		return nil, model.Fail("query_denied", "MongoDB queries use the configured database")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q.Cursor != "" {
		c.mu.Lock()
		p := c.cursors[q.Cursor]
		delete(c.cursors, q.Cursor)
		if p != nil {
			p.timer.Stop()
		}
		c.mu.Unlock()
		if p == nil {
			return nil, model.Fail("invalid_cursor", "MongoDB cursor expired or already consumed")
		}
		return c.page(ctx, p.cursor, l)
	}
	coll := c.client.Database(c.s.Database).Collection(q.Object)
	filter, e := mongoDocument(q.Filter)
	if e != nil {
		return nil, e
	}
	r := model.NewResult("documents")
	var cur *mongo.Cursor
	switch q.Operation {
	case "", "find":
		projection, e := mongoDocument(q.Projection)
		if e != nil {
			return nil, e
		}
		sort, e := mongoDocument(q.Sort)
		if e != nil {
			return nil, e
		}
		o := options.Find().SetBatchSize(int32(min(l.MaxRows, 100)))
		if len(projection) > 0 {
			o.SetProjection(projection)
		}
		if len(sort) > 0 {
			o.SetSort(sort)
		}
		cur, e = coll.Find(ctx, filter, o)
		if e != nil {
			return nil, e
		}
	case "aggregate":
		var pipeline []bson.D
		if len(q.Pipeline) == 0 {
			return nil, errors.New("pipeline is required")
		}
		if e = bson.UnmarshalExtJSON(q.Pipeline, false, &pipeline); e != nil {
			return nil, e
		}
		if e = guardMongo(pipeline); e != nil {
			return nil, e
		}
		cur, e = coll.Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(false).SetBatchSize(100))
		if e != nil {
			return nil, e
		}
	case "count":
		n, e := coll.CountDocuments(ctx, filter)
		if e != nil {
			return nil, e
		}
		_, e = r.Add(map[string]any{"count": value(n)}, l)
		return r, e
	case "distinct":
		if q.Query == "" || strings.ContainsAny(q.Query, "$\x00") {
			return nil, errors.New("query must name the distinct field")
		}
		// Unwind array values and exclude missing/empty fields, while retaining
		// explicit nulls. Grouping the field directly changes native distinct semantics.
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: filter}},
			{{Key: "$project", Value: bson.D{{Key: "value", Value: "$" + q.Query}}}},
			{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$value"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}},
			{{Key: "$match", Value: bson.D{{Key: "value", Value: bson.D{{Key: "$exists", Value: true}}}}}},
			{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$value"}}}},
		}
		cur, e = coll.Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(false).SetBatchSize(100))
		if e != nil {
			return nil, e
		}
	default:
		return nil, model.Fail("query_denied", "unsupported MongoDB operation")
	}
	return c.page(ctx, cur, l)
}

type mongoPage struct {
	cursor *mongo.Cursor
	timer  *time.Timer
}

func closeMongoCursor(cur *mongo.Cursor) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = cur.Close(ctx)
}
func (c *mongoConn) page(ctx context.Context, cur *mongo.Cursor, l model.Limits) (*model.Result, error) {
	retained := false
	defer func() {
		if !retained {
			closeMongoCursor(cur)
		}
	}()
	r := model.NewResult("documents")
	for r.RowCount < l.MaxRows && cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		v, err := mongoValue(d)
		if err != nil {
			return nil, err
		}
		ok, err := r.Add(v, l)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	if !r.Truncated && (cur.ID() != 0 || cur.RemainingBatchLength() > 0) {
		c.mu.Lock()
		defer c.mu.Unlock()
		// Bound server cursors even when a client abandons every first page.
		if len(c.cursors) >= 64 {
			return nil, model.Fail("busy", "MongoDB cursor capacity reached; retry after existing cursors expire")
		}
		id := secure.Random(24)
		p := &mongoPage{cursor: cur}
		p.timer = time.AfterFunc(5*time.Minute, func() {
			c.mu.Lock()
			current := c.cursors[id]
			delete(c.cursors, id)
			c.mu.Unlock()
			if current != nil {
				closeMongoCursor(current.cursor)
			}
		})
		c.cursors[id] = p
		r.NextCursor = id
		retained = true
	}
	return r, nil
}
func (c *mongoConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if op == "namespaces" {
		return []model.Object{{Name: c.s.Database, Type: "database"}}, nil
	}
	if ns != "" && ns != c.s.Database {
		return nil, model.Fail("query_denied", "namespace is outside the configured database")
	}
	out := []model.Object{}
	if op == "describe" {
		cur, e := c.client.Database(c.s.Database).Collection(obj).Indexes().List(ctx)
		if e != nil {
			return nil, e
		}
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var d bson.D
			if e = cur.Decode(&d); e != nil {
				return nil, e
			}
			v, e := mongoValue(d)
			if e != nil {
				return nil, e
			}
			o := model.Object{Name: obj, Namespace: c.s.Database, Type: "index", Details: v}
			for _, element := range d {
				if element.Key == "key" {
					if keys, ok := element.Value.(bson.D); ok {
						for _, field := range keys {
							o.Columns = append(o.Columns, model.Column{Name: field.Key, Type: "unknown (indexed field)"})
						}
					}
				}
			}
			out = append(out, o)
			if len(out) >= 1000 {
				break
			}
		}
		return out, cur.Err()
	}
	names, e := c.client.Database(c.s.Database).ListCollectionNames(ctx, bson.D{})
	if e != nil {
		return nil, e
	}
	for _, name := range names {
		out = append(out, model.Object{Name: name, Namespace: c.s.Database, Type: "collection"})
		if len(out) >= 1000 {
			break
		}
	}
	return out, nil
}
