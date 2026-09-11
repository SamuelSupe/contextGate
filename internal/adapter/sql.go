package adapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	duckdb "github.com/duckdb/duckdb-go/v2"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	sqlite3 "github.com/mattn/go-sqlite3"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type sqlConn struct {
	db *sql.DB
	s  model.Source
}

var sqliteOnce sync.Once

func tlsConfig(s model.Source) (*tls.Config, error) {
	if s.TLSMode == "disable" {
		return nil, nil
	}
	c := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: strings.Split(s.Host, ",")[0]}
	if s.CACert != "" {
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(s.CACert)) {
			return nil, errors.New("invalid CA certificate")
		}
		c.RootCAs = roots
	}
	return c, nil
}
func openSQL(ctx context.Context, s model.Source) (Connection, error) {
	var db *sql.DB
	var e error
	tc, e := tlsConfig(s)
	if e != nil {
		return nil, e
	}
	switch s.Kind {
	case "postgres", "timescaledb", "cockroachdb":
		u := &url.URL{Scheme: "postgres", Host: address(s), Path: "/" + s.Database, User: url.UserPassword(s.Username, s.Password)}
		p := u.Query()
		p.Set("sslmode", "disable")
		u.RawQuery = p.Encode()
		cfg, err := pgx.ParseConfig(u.String())
		if err != nil {
			return nil, err
		}
		cfg.TLSConfig = tc
		cfg.ConnectTimeout = 10 * time.Second
		cfg.RuntimeParams["application_name"] = "mcpdbhub"
		db = stdlib.OpenDB(*cfg)
	case "mysql", "mariadb", "tidb":
		cfg := mysql.NewConfig()
		cfg.User = s.Username
		cfg.Passwd = s.Password
		cfg.Net = "tcp"
		cfg.Addr = address(s)
		cfg.DBName = s.Database
		cfg.TLS = tc
		cfg.Timeout = 10 * time.Second
		cfg.ReadTimeout = 130 * time.Second
		cfg.WriteTimeout = 10 * time.Second
		cfg.MultiStatements = false
		cfg.AllowAllFiles = false
		cfg.AllowNativePasswords = true
		con, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, err
		}
		db = sql.OpenDB(con)
	case "clickhouse":
		db = clickhouse.OpenDB(&clickhouse.Options{Addr: []string{address(s)}, Auth: clickhouse.Auth{Database: s.Database, Username: s.Username, Password: s.Password}, TLS: tc, Settings: clickhouse.Settings{"readonly": 1, "allow_ddl": 0, "allow_introspection_functions": 0, "max_execution_time": s.Limits.TimeoutSeconds, "max_block_size": 256, "max_result_bytes": s.Limits.MaxBytes, "result_overflow_mode": "throw"}, DialTimeout: 10 * time.Second})
	case "sqlite":
		sqliteOnce.Do(func() {
			sql.Register("sqlite3_readonly", &sqlite3.SQLiteDriver{ConnectHook: func(c *sqlite3.SQLiteConn) error {
				c.RegisterAuthorizer(func(op int, a, b, c string) int {
					switch op {
					case sqlite3.SQLITE_SELECT, sqlite3.SQLITE_READ, 33:
						return sqlite3.SQLITE_OK
					case sqlite3.SQLITE_FUNCTION:
						if pureFunctions[strings.ToLower(b)] || wordset("like glob likelihood likely unlikely total instr unicode char soundex sqlite_version")[strings.ToLower(b)] {
							return sqlite3.SQLITE_OK
						}
					case sqlite3.SQLITE_PRAGMA:
						if wordset("table_info table_xinfo index_info index_list database_list")[a] || a == "query_only" && b == "" {
							return sqlite3.SQLITE_OK
						}
					}
					return sqlite3.SQLITE_DENY
				})
				return nil
			}})
		})
		u := &url.URL{Scheme: "file", Path: s.Path}
		p := u.Query()
		p.Set("mode", "ro")
		p.Set("_query_only", "1")
		p.Set("_busy_timeout", "5000")
		u.RawQuery = p.Encode()
		db, e = sql.Open("sqlite3_readonly", u.String())
	case "duckdb":
		p := url.Values{"access_mode": {"READ_ONLY"}, "enable_external_access": {"false"}, "autoload_known_extensions": {"false"}, "autoinstall_known_extensions": {"false"}, "allow_unsigned_extensions": {"false"}, "memory_limit": {"512MB"}, "threads": {"2"}}
		con, err := duckdb.NewConnector(s.Path+"?"+p.Encode(), func(ex driver.ExecerContext) error {
			_, err := ex.ExecContext(context.Background(), "SET lock_configuration = true", nil)
			return err
		})
		if err != nil {
			return nil, err
		}
		db = sql.OpenDB(con)
	default:
		return nil, errors.New("unsupported SQL kind")
	}
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(s.Limits.Concurrency)
	db.SetMaxIdleConns(s.Limits.Concurrency)
	db.SetConnMaxLifetime(10 * time.Minute)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	c := &sqlConn{db, s}
	if s.Kind == "tidb" {
		ok, e := c.selectOnly(ctx)
		if e != nil || !ok {
			db.Close()
			return nil, model.Fail("readonly_required", "TiDB requires a SELECT-only account; inspect SHOW GRANTS")
		}
	}
	return c, nil
}
func (c *sqlConn) Close() error { return c.db.Close() }
func (c *sqlConn) selectOnly(ctx context.Context) (bool, error) {
	rows, e := c.db.QueryContext(ctx, "SHOW GRANTS")
	if e != nil {
		return false, e
	}
	defer rows.Close()
	seen := false
	for rows.Next() {
		var grant string
		if e = rows.Scan(&grant); e != nil {
			return false, e
		}
		g := strings.ToUpper(grant)
		if strings.Contains(g, "GRANT USAGE ") {
			continue
		}
		if !strings.HasPrefix(g, "GRANT SELECT") && !strings.HasPrefix(g, "GRANT SHOW VIEW") {
			return false, nil
		}
		if strings.Contains(g, "WITH GRANT OPTION") {
			return false, nil
		}
		part := strings.SplitN(strings.TrimPrefix(g, "GRANT "), " ON ", 2)
		if len(part) != 2 {
			return false, nil
		}
		for _, priv := range strings.Split(part[0], ",") {
			p := strings.TrimSpace(priv)
			if p != "SELECT" && p != "SHOW VIEW" {
				return false, nil
			}
		}
		seen = true
	}
	return seen, rows.Err()
}
func (c *sqlConn) Probe(ctx context.Context) (model.Probe, error) {
	cap, _ := Get(c.s.Kind)
	p := model.Probe{Connected: true, Protection: cap.Protection, PermissionStatus: "unverified", Evidence: []string{}, CheckedAt: time.Now()}
	query := "SELECT version()"
	switch c.s.Kind {
	case "sqlite":
		query = "SELECT sqlite_version()"
	case "mysql", "mariadb", "tidb":
		query = "SELECT VERSION()"
	}
	if e := c.db.QueryRowContext(ctx, query).Scan(&p.ServerVersion); e != nil {
		return p, e
	}
	if c.s.Kind == "timescaledb" {
		var extension string
		if err := c.db.QueryRowContext(ctx, "SELECT extversion FROM pg_extension WHERE extname='timescaledb'").Scan(&extension); err != nil {
			return p, errors.New("TimescaleDB extension is not installed")
		}
		p.ServerVersion = "TimescaleDB " + extension + " / " + p.ServerVersion
	}
	switch c.s.Kind {
	case "postgres", "timescaledb", "cockroachdb":
		tx, e := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return p, e
		}
		defer tx.Rollback()
		var mode string
		if e = tx.QueryRowContext(ctx, "SHOW transaction_read_only").Scan(&mode); e != nil {
			return p, e
		}
		if mode != "on" {
			return p, errors.New("read-only transaction was not enabled")
		}
		p.PermissionStatus = "engine_enforced"
		p.Evidence = append(p.Evidence, "transaction_read_only=on for every query; account grants are not inferred")
	case "mysql", "mariadb":
		tx, e := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return p, e
		}
		tx.Rollback()
		p.PermissionStatus = "engine_enforced"
		p.Evidence = append(p.Evidence, "START TRANSACTION READ ONLY accepted; every query uses a new read-only transaction")
	case "tidb":
		p.PermissionStatus = "verified"
		p.Evidence = append(p.Evidence, "SHOW GRANTS contains only SELECT / SHOW VIEW / USAGE")
	case "sqlite", "duckdb":
		p.PermissionStatus = "engine_enforced"
		p.Evidence = append(p.Evidence, "Database opened READ_ONLY; external file access and extensions disabled")
	case "clickhouse":
		var ro string
		if e := c.db.QueryRowContext(ctx, "SELECT value FROM system.settings WHERE name='readonly'").Scan(&ro); e != nil {
			return p, e
		}
		if ro != "1" {
			return p, errors.New("ClickHouse readonly setting was not enforced")
		}
		p.PermissionStatus = "engine_enforced"
		p.Evidence = append(p.Evidence, "readonly=1 and allow_ddl=0 enforced in connection settings")
	}
	return p, nil
}
func (c *sqlConn) Query(ctx context.Context, q model.Query, l model.Limits) (*model.Result, error) {
	if e := guardSQL(c.s.Kind, q.Query); e != nil {
		return nil, e
	}
	conn, e := c.db.Conn(ctx)
	if e != nil {
		return nil, e
	}
	defer conn.Close()
	if c.s.Kind == "duckdb" {
		e = conn.Raw(func(raw any) error {
			d, ok := raw.(*duckdb.Conn)
			if !ok {
				return errors.New("unexpected DuckDB driver")
			}
			stmt, e := d.PrepareContext(ctx, q.Query)
			if e != nil {
				return e
			}
			defer stmt.Close()
			typ, e := stmt.(*duckdb.Stmt).StatementType()
			if e != nil {
				return e
			}
			if typ != duckdb.STATEMENT_TYPE_SELECT {
				return model.Fail("query_denied", "DuckDB statement is not SELECT")
			}
			return nil
		})
		if e != nil {
			return nil, e
		}
	}
	var rows *sql.Rows
	args := params(q.Params)
	if len(q.NamedParams) > 0 {
		if c.s.Kind == "clickhouse" {
			p := clickhouse.Parameters{}
			for k, v := range q.NamedParams {
				p[k] = fmt.Sprint(v)
			}
			ctx = clickhouse.Context(ctx, clickhouse.WithParameters(p))
		} else {
			for k, v := range q.NamedParams {
				args = append(args, sql.Named(k, params([]any{v})[0]))
			}
		}
	}
	if c.s.Kind == "postgres" || c.s.Kind == "timescaledb" || c.s.Kind == "cockroachdb" || c.s.Kind == "mysql" || c.s.Kind == "mariadb" {
		tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		if c.s.Kind == "postgres" || c.s.Kind == "timescaledb" {
			if _, err = tx.ExecContext(ctx, "SELECT set_config('statement_timeout',$1,true),set_config('search_path','pg_catalog,public',true)", strconv.Itoa(l.TimeoutSeconds*1000)); err != nil {
				return nil, err
			}
		}
		rows, e = tx.QueryContext(ctx, q.Query, args...)
	} else {
		rows, e = conn.QueryContext(ctx, q.Query, args...)
	}
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	return scanRows(rows, l)
}
func scanRows(rows *sql.Rows, l model.Limits) (*model.Result, error) {
	types, e := rows.ColumnTypes()
	if e != nil {
		return nil, e
	}
	r := model.NewResult("table")
	for _, t := range types {
		r.Columns = append(r.Columns, model.Column{Name: t.Name(), Type: t.DatabaseTypeName()})
	}
	for rows.Next() {
		raw := make([]any, len(types))
		ptr := make([]any, len(types))
		for i := range ptr {
			ptr[i] = &raw[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			return nil, e
		}
		for i, v := range raw {
			if b, ok := v.([]byte); ok {
				typ := strings.ToUpper(types[i].DatabaseTypeName())
				if !strings.Contains(typ, "BLOB") && !strings.Contains(typ, "BINARY") && typ != "BYTEA" {
					raw[i] = value(string(b))
					continue
				}
			}
			raw[i] = value(v)
		}
		ok, e := r.Add(raw, l)
		if e != nil {
			return nil, e
		}
		if !ok {
			break
		}
	}
	return r, rows.Err()
}
