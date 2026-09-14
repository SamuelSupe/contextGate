package adapter

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/redis/go-redis/v9"
	"net"
	"strconv"
	"strings"
	"time"
)

type redisConn struct{ s model.Source }

func (c *redisConn) client(limit int) (redis.UniversalClient, error) {
	tc, e := tlsConfig(c.s)
	if e != nil {
		return nil, e
	}
	db := 0
	if c.s.Database != "" {
		db, e = strconv.Atoi(c.s.Database)
		if e != nil || db < 0 {
			return nil, errors.New("Redis database must be a nonnegative integer")
		}
	}
	hosts := strings.Split(c.s.Host, ",")
	for i, h := range hosts {
		s := c.s
		s.Host = h
		hosts[i] = address(s)
	}
	o := &redis.UniversalOptions{Addrs: hosts, Username: c.s.Username, Password: c.s.Password, DB: db, TLSConfig: tc, PoolSize: 1, MaxRetries: -1, DialTimeout: 5 * time.Second, ReadTimeout: time.Duration(c.s.Limits.TimeoutSeconds) * time.Second, ContextTimeoutEnabled: true, MasterName: c.s.Options["sentinel_master"], Protocol: 2}
	o.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{Timeout: 5 * time.Second}
		var conn net.Conn
		var err error
		if tc != nil {
			conn, err = (&tls.Dialer{NetDialer: &d, Config: tc}).DialContext(ctx, network, addr)
		} else {
			conn, err = d.DialContext(ctx, network, addr)
		}
		if err != nil {
			return nil, err
		}
		return newReadBudgetConn(conn, limit+64*1024), nil
	}
	return redis.NewUniversalClient(o), nil
}
func openRedis(ctx context.Context, s model.Source) (Connection, error) {
	c := &redisConn{s}
	client, e := c.client(1 << 20)
	if e != nil {
		return nil, e
	}
	defer client.Close()
	if e = client.Ping(ctx).Err(); e != nil {
		return nil, e
	}
	return c, nil
}
func (c *redisConn) Close() error { return nil }
func (c *redisConn) Probe(ctx context.Context) (model.Probe, error) {
	p := model.Probe{Connected: true, Protection: "command_allowlist", PermissionStatus: "unverified", Evidence: []string{"Explicit read command allowlist; database account permissions have not been inferred"}, CheckedAt: time.Now()}
	client, e := c.client(1 << 20)
	if e != nil {
		return p, e
	}
	defer client.Close()
	if info, e := client.Info(ctx, "server").Result(); e == nil {
		for _, line := range strings.Split(info, "\n") {
			if strings.HasPrefix(line, "redis_version:") || strings.HasPrefix(line, "valkey_version:") {
				p.ServerVersion = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				if c.s.Kind == "redis" || strings.HasPrefix(line, "valkey_version:") {
					break
				}
			}
		}
	}
	return p, nil
}

var redisAllowed = wordset("GET MGET STRLEN GETRANGE EXISTS TYPE TTL PTTL HGET HMGET HLEN HEXISTS HSTRLEN HSCAN LLEN LINDEX LRANGE SCARD SISMEMBER SMISMEMBER SSCAN ZCARD ZCOUNT ZLEXCOUNT ZRANK ZREVRANK ZSCORE ZMSCORE ZRANGE ZREVRANGE ZRANGEBYSCORE ZREVRANGEBYSCORE ZSCAN SCAN XLEN XRANGE XREVRANGE")

func (c *redisConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	cmd := strings.ToUpper(q.Command)
	if !redisAllowed[strings.ToLower(cmd)] {
		return nil, model.Fail("query_denied", "Redis command is not in the read allowlist")
	}
	if len(q.Args) > 10000 {
		return nil, errors.New("too many arguments")
	}
	args := append([]string{}, q.Args...)
	scan := strings.HasSuffix(cmd, "SCAN")
	if scan {
		pos := 0
		if cmd != "SCAN" {
			pos = 1
		}
		if len(args) <= pos {
			return nil, errors.New("scan cursor argument is required")
		}
		if q.Cursor != "" {
			args[pos] = q.Cursor
		}
		found := false
		for i := pos + 1; i < len(args); i++ {
			if strings.EqualFold(args[i], "COUNT") {
				if i+1 >= len(args) {
					return nil, errors.New("COUNT value required")
				}
				n, e := strconv.Atoi(args[i+1])
				if e != nil || n < 1 {
					return nil, errors.New("invalid COUNT")
				}
				if n > l.MaxRows {
					args[i+1] = strconv.Itoa(l.MaxRows)
				}
				found = true
			}
		}
		if !found {
			args = append(args, "COUNT", strconv.Itoa(l.MaxRows))
		}
	}
	client, e := c.client(l.MaxBytes)
	if e != nil {
		return nil, e
	}
	defer client.Close()
	a := []any{cmd}
	for _, arg := range args {
		a = append(a, arg)
	}
	v, e := client.Do(ctx, a...).Result()
	if errors.Is(e, redis.Nil) {
		v = nil
		e = nil
	}
	if e != nil {
		return nil, e
	}
	r := model.NewResult("values")
	if scan {
		pair, ok := v.([]any)
		if !ok || len(pair) != 2 {
			return nil, errors.New("invalid scan response")
		}
		cursor := fmt.Sprint(pair[0])
		if cursor != "0" {
			r.NextCursor = cursor
		}
		v = pair[1]
	}
	if list, ok := v.([]any); ok {
		for _, item := range list {
			ok, e := r.Add(value(item), l)
			if e != nil {
				return nil, e
			}
			if !ok {
				r.NextCursor = ""
				break
			}
		}
	} else {
		if _, e = r.Add(value(v), l); e != nil {
			return nil, e
		}
	}
	return r, nil
}
func (c *redisConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	if op == "namespaces" {
		return []model.Object{{Name: c.s.Database, Type: "database"}}, nil
	}
	if ns != "" && ns != c.s.Database {
		return nil, model.Fail("query_denied", "namespace is outside configured Redis database")
	}
	client, e := c.client(1 << 20)
	if e != nil {
		return nil, e
	}
	defer client.Close()
	if op == "describe" {
		typ, e := client.Type(ctx, obj).Result()
		if e != nil {
			return nil, e
		}
		return []model.Object{{Name: obj, Type: typ, Namespace: c.s.Database}}, nil
	}
	keys, _, e := client.Scan(ctx, 0, "*", 100).Result()
	if e != nil {
		return nil, e
	}
	out := []model.Object{}
	for _, k := range keys {
		out = append(out, model.Object{Name: k, Type: "key", Namespace: c.s.Database})
	}
	return out, nil
}
