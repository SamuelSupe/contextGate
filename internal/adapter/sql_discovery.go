package adapter

import (
	"context"
	"database/sql"
	"errors"
	"github.com/SamuelSupe/contextGate/internal/model"
	"strconv"
	"strings"
)

func (c *sqlConn) Discover(ctx context.Context, op, ns, obj string) ([]model.Object, error) {
	return c.discoverRows(ctx, op, ns, obj, 10001, 0)
}

func (c *sqlConn) DiscoverPage(ctx context.Context, op string, q model.Query, l model.Limits) (*model.Result, error) {
	offset := 0
	if q.Cursor != "" {
		value, err := strconv.ParseUint(q.Cursor, 10, 32)
		if err != nil {
			return nil, model.Fail("invalid_cursor", "invalid metadata cursor")
		}
		offset = int(value)
	}
	objects, err := c.discoverRows(ctx, op, q.Namespace, q.Object, l.MaxRows+1, offset)
	if err != nil {
		return nil, err
	}
	r := model.NewResult("metadata")
	budget := l
	if len(objects) > 1 {
		// The execution layer wraps the numeric offset in an encrypted cursor,
		// included in both MCP's structured result and JSON text fallback.
		budget.MaxBytes -= 768
	}
	for _, object := range objects {
		ok, err := r.Add(object, budget)
		if err != nil {
			return nil, err
		}
		if !ok {
			if r.RowCount == 0 {
				return nil, model.Fail("result_too_large", "one metadata object exceeds the response byte limit")
			}
			r.NextCursor = strconv.Itoa(offset + r.RowCount)
			r.Truncated = false
			break
		}
	}
	return r, nil
}

func (c *sqlConn) discoverRows(ctx context.Context, op, ns, obj string, limit, offset int) ([]model.Object, error) {
	out := []model.Object{}
	var rows *sql.Rows
	var e error
	query := func(statement string, args ...any) (*sql.Rows, error) {
		return c.db.QueryContext(ctx, statement+" LIMIT "+strconv.Itoa(limit)+" OFFSET "+strconv.Itoa(offset), args...)
	}
	ispg := c.s.Kind == "postgres" || c.s.Kind == "timescaledb" || c.s.Kind == "cockroachdb"
	switch c.s.Kind {
	case "redshift":
		if ns == "" {
			ns = "public"
		}
		switch op {
		case "namespaces":
			rows, e = query("SELECT schema_name, 'schema' FROM svv_redshift_schemas WHERE database_name=current_database() ORDER BY schema_name")
		case "objects":
			rows, e = query("SELECT table_name, table_type FROM svv_redshift_tables WHERE database_name=current_database() AND schema_name=$1 ORDER BY table_name", ns)
		case "describe":
			rows, e = query("SELECT column_name, data_type FROM svv_redshift_columns WHERE database_name=current_database() AND schema_name=$1 AND table_name=$2 ORDER BY ordinal_position", ns, obj)
		default:
			return nil, errors.New("unknown discovery operation")
		}

	case "sqlite":
		if op == "namespaces" {
			if offset > 0 {
				return out, nil
			}
			return []model.Object{{Name: "main", Type: "database"}}, nil
		}
		if ns != "" && ns != "main" {
			return nil, errors.New("unknown namespace")
		}
		if op == "describe" {
			rows, e = query("SELECT name,type FROM pragma_table_info(?) ORDER BY cid", obj)
		} else {
			rows, e = query("SELECT name,type FROM sqlite_schema WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name")
		}
	case "duckdb":
		if op == "namespaces" {
			rows, e = query("SELECT schema_name, 'schema' FROM information_schema.schemata ORDER BY schema_name")
		} else if op == "describe" {
			if ns == "" {
				ns = "main"
			}
			rows, e = query("SELECT column_name,data_type FROM information_schema.columns WHERE table_schema=? AND table_name=? ORDER BY ordinal_position", ns, obj)
		} else {
			if ns == "" {
				ns = "main"
			}
			rows, e = query("SELECT table_name,table_type FROM information_schema.tables WHERE table_schema=? ORDER BY table_name", ns)
		}
	case "clickhouse":
		if op == "namespaces" {
			rows, e = query("SELECT name, 'database' FROM system.databases ORDER BY name")
		} else if op == "describe" {
			if ns == "" {
				ns = c.s.Database
			}
			rows, e = query("SELECT name,type FROM system.columns WHERE database=? AND table=? ORDER BY position", ns, obj)
		} else {
			if ns == "" {
				ns = c.s.Database
			}
			rows, e = query("SELECT name,engine FROM system.tables WHERE database=? ORDER BY name", ns)
		}
	default:
		bind := func(q string) string {
			if ispg {
				q = strings.Replace(q, "?", "$1", 1)
				q = strings.Replace(q, "?", "$2", 1)
			}
			return q
		}
		if op == "namespaces" {
			statement := "SELECT schema_name, 'schema' FROM information_schema.schemata"
			if ispg {
				statement += " WHERE has_schema_privilege(schema_name,'USAGE')"
			}
			rows, e = query(statement + " ORDER BY schema_name")
		} else if op == "describe" {
			if ns == "" {
				ns = c.s.Database
				if ispg {
					ns = "public"
				}
			}
			rows, e = query(bind("SELECT column_name,data_type FROM information_schema.columns WHERE table_schema=? AND table_name=? ORDER BY ordinal_position"), ns, obj)
		} else {
			if ns == "" {
				ns = c.s.Database
				if ispg {
					ns = "public"
				}
			}
			rows, e = query(bind("SELECT table_name,table_type FROM information_schema.tables WHERE table_schema=? ORDER BY table_name"), ns)
		}
	}
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var name, typ string
		if e = rows.Scan(&name, &typ); e != nil {
			return nil, e
		}
		out = append(out, model.Object{Name: name, Namespace: ns, Type: typ})
	}
	return out, rows.Err()
}
