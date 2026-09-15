package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	chparser "github.com/AfterShip/clickhouse-sql-parser/parser"
	"github.com/SamuelSupe/contextGate/internal/model"
	pgquery "github.com/pganalyze/pg_query_go/v6"
	mysqlparser "github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
	"strings"
	"unicode"
)

type token struct {
	text   string
	quoted bool
}

func lex(q string) ([]token, error) {
	out := []token{}
	depth := 0
	for i := 0; i < len(q); {
		c := q[i]
		if unicode.IsSpace(rune(c)) {
			i++
			continue
		}
		if c == '-' && i+1 < len(q) && q[i+1] == '-' {
			for i < len(q) && q[i] != '\n' {
				i++
			}
			continue
		}
		if c == '#' {
			for i < len(q) && q[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(q) && q[i+1] == '*' {
			if i+2 < len(q) && (q[i+2] == '!' || q[i+2] == 'M' || q[i+2] == 'm') {
				return nil, errors.New("executable comments are denied")
			}
			i += 2
			n := 1
			for i < len(q) && n > 0 {
				if i+1 < len(q) && q[i:i+2] == "/*" {
					n++
					i += 2
				} else if i+1 < len(q) && q[i:i+2] == "*/" {
					n--
					i += 2
				} else {
					i++
				}
			}
			if n != 0 {
				return nil, errors.New("unterminated comment")
			}
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote := c
			start := i
			i++
			closed := false
			for i < len(q) {
				if q[i] == '\\' {
					return nil, errors.New("use parameters instead of backslash escapes")
				}
				if q[i] == quote {
					if i+1 < len(q) && q[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, errors.New("unterminated quote")
			}
			t := q[start+1 : i-1]
			if quote == '\'' {
				t = "literal"
			}
			out = append(out, token{strings.ToLower(t), true})
			continue
		}
		if c == '$' {
			j := i + 1
			for j < len(q) && ((q[j] >= 'a' && q[j] <= 'z') || (q[j] >= 'A' && q[j] <= 'Z') || q[j] == '_') {
				j++
			}
			if j < len(q) && q[j] == '$' {
				tag := q[i : j+1]
				end := strings.Index(q[j+1:], tag)
				if end < 0 {
					return nil, errors.New("unterminated dollar string")
				}
				i = j + 1 + end + len(tag)
				out = append(out, token{"literal", true})
				continue
			}
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			j := i + 1
			for j < len(q) && ((q[j] >= 'a' && q[j] <= 'z') || (q[j] >= 'A' && q[j] <= 'Z') || (q[j] >= '0' && q[j] <= '9') || q[j] == '_' || q[j] == '$') {
				j++
			}
			out = append(out, token{strings.ToLower(q[i:j]), false})
			i = j
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
			if depth < 0 {
				return nil, errors.New("unbalanced parentheses")
			}
		}
		out = append(out, token{string(c), false})
		i++
	}
	if depth != 0 {
		return nil, errors.New("unbalanced parentheses")
	}
	if len(out) > 0 && out[len(out)-1].text == ";" {
		out = out[:len(out)-1]
	}
	for _, t := range out {
		if t.text == ";" && !t.quoted {
			return nil, errors.New("multiple statements are denied")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("empty query")
	}
	return out, nil
}

var deniedSQL = wordset("insert update delete replace upsert merge create alter drop truncate grant revoke copy attach detach vacuum analyze execute exec call do set reset use load install import export into outfile dumpfile lock kill optimize system begin commit rollback savepoint pragma settings format")
var pureFunctions = wordset(`abs acos asin atan atan2 ceil ceiling floor round trunc truncate sqrt cbrt pow power exp ln log log10 log2 mod sign pi degrees radians sin cos tan cot greatest least coalesce nullif if ifnull iif nvl isnull count sum avg min max std stddev stddev_pop stddev_samp variance var_pop var_samp median percentile_cont percentile_disc approx_count_distinct count_if sumif countif avgif minif maxif uniq uniqexact quantile quantiles grouparray groupuniqarray group_concat string_agg array_agg json_agg jsonb_agg json_object_agg jsonb_object_agg bit_and bit_or bit_xor bool_and bool_or every any_value argmin argmax arg_min arg_max
 length char_length character_length octet_length bit_length lower upper initcap trim ltrim rtrim btrim substring substr left right replace concat concat_ws position strpos instr locate reverse repeat lpad rpad split_part starts_with ends_with regexp_replace regexp_match regexp_matches regexp_like regexp_extract regexp_substr regexp_count match extract extractall splitbychar splitbystring arraystringconcat format formatdatetime
 now current_timestamp current_date current_time localtimestamp date time timestamp datetime julianday strftime unixepoch date_trunc date_part date_bin date_format date_add date_sub date_diff datediff timediff timestampdiff timestampadd to_timestamp to_date to_char age timezone extract make_date make_timestamp make_interval time_bucket time_bucket_ng first last locf interpolate time_weight integral counter_agg delta rate derivative elapsed moving_average non_negative_derivative difference non_negative_difference cumulative_sum fill distinct spread mode stddev sample top bottom elapseddate
 row_number rank dense_rank percent_rank cume_dist ntile lag lead first_value last_value nth_value
 json_extract json_extract_path json_extract_path_text jsonb_extract_path jsonb_extract_path_text json_array_length jsonb_array_length json_type json_typeof jsonb_typeof json_valid json_quote json_unquote json_build_object jsonb_build_object json_build_array jsonb_build_array json_object json_array json_contains json_contains_path json_value json_query json_length json_keys json_each jsonb_each jsonb_each_text json_array_elements jsonb_array_elements json_array_elements_text jsonb_array_elements_text to_json to_jsonb
 array_length cardinality array_position array_positions array_append array_prepend array_cat array_remove array_replace array_to_string string_to_array unnest generate_series generate_subscripts range list_value list_contains list_extract list_sort array_sort array_distinct array_join arrayjoin arraymap arrayfilter arrayexists arrayall arraycount arraysum arrayavg arraymin arraymax arrayelement arrayenumerate arrayzip arrayslice has hasany hasall indexof tuple tupleelement map mapkeys mapvalues
 cast convert try_cast try_convert typeof pg_typeof version sqlite_version current_database current_schema current_schemas current_user session_user database user connection_id toint8 toint16 toint32 toint64 touint8 touint16 touint32 touint64 tofloat32 tofloat64 todecimal32 todecimal64 todecimal128 tostring todate todatetime todatetime64 tostartofhour tostartofday tostartofweek tostartofmonth tostartofyear tostartofinterval toyear tomonth today yesterday tohour tominute tosecond tounixtimestamp parseDateTimeBestEffort
 hex unhex encode decode md5 sha1 sha2 sha256 sha512 digest uuid uuidtoblob blobtouuid tojson fromjson countdistinct dateof unixtimestampof mintimeuuid maxtimeuuid ttl writetime token blobastext textasblob asciiasblob blobasascii type tags properties labels id elementid nodes relationships size head tail keys collect exists isfinite isnan isinf roundbankers
 sumover countover avgMerge sumMerge countMerge uniqMerge`)

func wordset(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		out[w] = true
	}
	return out
}
func guardSQL(kind, q string) error {
	ts, e := lex(q)
	if e != nil {
		return model.Fail("query_denied", e.Error())
	}
	if ts[0].text != "select" && ts[0].text != "with" {
		return model.Fail("query_denied", "only SELECT queries are accepted")
	}
	for i, t := range ts {
		if !t.quoted && deniedSQL[t.text] {
			// REPLACE, TRUNCATE and FORMAT are also pure scalar functions.
			// Statement parsing / engine classification still checks the enclosing query.
			if pureFunctions[t.text] && i+1 < len(ts) && ts[i+1].text == "(" {
				continue
			}
			return model.Fail("query_denied", "statement contains a forbidden operation")
		}
	}
	switch kind {
	case "snowflake", "databricks", "bigquery":
		return guardCloudSQL(kind, q)
	case "postgres", "timescaledb", "cockroachdb", "redshift":
		b, e := pgquery.ParseToJSON(q)
		if e != nil {
			return model.Fail("invalid_query", "query could not be parsed as PostgreSQL SQL")
		}
		var root map[string]any
		if e = json.Unmarshal([]byte(b), &root); e != nil {
			return e
		}
		stmts, _ := root["stmts"].([]any)
		if len(stmts) != 1 {
			return model.Fail("query_denied", "exactly one statement is required")
		}
		stmt := stmts[0].(map[string]any)["stmt"].(map[string]any)
		if _, ok := stmt["SelectStmt"]; !ok {
			return model.Fail("query_denied", "only SELECT is accepted")
		}
		return walkPG(root)
	case "mysql", "mariadb", "tidb":
		nodes, _, e := mysqlparser.New().ParseSQL(q)
		if e != nil || len(nodes) != 1 {
			return model.Fail("invalid_query", "query could not be parsed as a single MySQL SELECT")
		}
		switch nodes[0].(type) {
		case *ast.SelectStmt, *ast.SetOprStmt:
		default:
			return model.Fail("query_denied", "only SELECT is accepted")
		}
		v := &sqlVisitor{}
		nodes[0].Accept(v)
		return v.err
	case "clickhouse":
		nodes, e := chparser.NewParser(q).ParseStmts()
		if e != nil || len(nodes) != 1 {
			return model.Fail("invalid_query", "query could not be parsed as a single ClickHouse SELECT")
		}
		if _, ok := nodes[0].(*chparser.SelectQuery); !ok {
			return model.Fail("query_denied", "only SELECT is accepted")
		}
		return guardCalls(ts)
	case "cassandra", "scylladb":
		if ts[0].text != "select" {
			return model.Fail("query_denied", "CQL requires SELECT")
		}
		return guardCalls(ts)
	}
	return nil
}
func walkPG(v any) error {
	switch n := v.(type) {
	case map[string]any:
		for k, x := range n {
			if k == "IntoClause" || k == "LockingClause" || k == "InsertStmt" || k == "UpdateStmt" || k == "DeleteStmt" || k == "MergeStmt" {
				return model.Fail("query_denied", "query contains a write or lock")
			}
			if k == "FuncCall" {
				fn, _ := x.(map[string]any)
				parts, _ := fn["funcname"].([]any)
				names := []string{}
				for _, part := range parts {
					p, _ := part.(map[string]any)
					s, _ := p["String"].(map[string]any)
					name, _ := s["sval"].(string)
					names = append(names, strings.ToLower(name))
				}
				if len(names) == 0 || !pureFunctions[names[len(names)-1]] || (len(names) > 1 && names[0] != "pg_catalog") {
					return model.Fail("query_denied", "function is not in the pure query allowlist")
				}
			}
			if e := walkPG(x); e != nil {
				return e
			}
		}
	case []any:
		for _, x := range n {
			if e := walkPG(x); e != nil {
				return e
			}
		}
	}
	return nil
}

type sqlVisitor struct{ err error }

func (v *sqlVisitor) Enter(n ast.Node) (ast.Node, bool) {
	// MySQL read-only transactions still allow shared locks. Inspect every
	// SELECT, including subqueries and CTEs, before it reaches the database.
	if selectStmt, ok := n.(*ast.SelectStmt); ok && selectStmt.LockInfo != nil && selectStmt.LockInfo.LockType != ast.SelectLockNone {
		v.err = model.Fail("query_denied", "locking reads are denied")
	}
	if f, ok := n.(*ast.FuncCallExpr); ok && (f.Schema.O != "" || !pureFunctions[strings.ToLower(f.FnName.O)]) {
		v.err = model.Fail("query_denied", "function is not in the pure query allowlist")
	}
	return n, v.err != nil
}
func (v *sqlVisitor) Leave(n ast.Node) (ast.Node, bool) { return n, v.err == nil }
func guardCalls(ts []token) error {
	return guardCallsAllowed(ts, pureFunctions)
}
func guardCallsAllowed(ts []token, allowed map[string]bool) error {
	structural := wordset("as in over select with values grouping sets rollup cube partition by tuple")
	for i := 0; i+1 < len(ts); i++ {
		t := ts[i]
		if ts[i+1].text != "(" {
			continue
		}
		if i > 0 && ts[i-1].text == "." {
			return model.Fail("query_denied", "qualified custom functions are denied")
		}
		if t.text == ")" || t.text == "(" || t.text == "," {
			continue
		}
		if len(t.text) == 0 || !(unicode.IsLetter(rune(t.text[0])) || t.text[0] == '_') {
			continue
		}
		if !allowed[t.text] && !structural[t.text] {
			return model.Fail("query_denied", fmt.Sprintf("function %s is not allowed", t.text))
		}
	}
	return nil
}
