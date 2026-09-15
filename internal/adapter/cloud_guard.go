package adapter

import (
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
)

// Only the validation copy is normalized. Native query text and parameter
// values are sent separately and unchanged. Reject ambiguous dialect quoting
// instead of trusting a parser that could see a different statement.
func guardCloudSQL(kind, query string) error {
	var out strings.Builder
	parameter := 0
	for i := 0; i < len(query); {
		ch := query[i]
		if ch == '\\' || ch == '$' || ch == '#' || ch == ';' || ch == 0 {
			return model.Fail("query_denied", "cloud SQL accepts one SELECT without scripts, escapes or session variables")
		}
		if ch == '-' && i+1 < len(query) && query[i+1] == '-' {
			for i < len(query) && query[i] != '\n' {
				i++
			}
			out.WriteByte(' ')
			continue
		}
		if ch == '/' && i+1 < len(query) && query[i+1] == '*' {
			return model.Fail("query_denied", "block comments and optimizer hints are outside the supported cloud SQL subset")
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			if ch == '"' && kind != "snowflake" || ch == '`' && kind == "snowflake" {
				return model.Fail("query_denied", "use the database's standard identifier quotes and single-quoted strings")
			}
			quote := ch
			if ch == '`' {
				quote = '"'
			}
			out.WriteByte(quote)
			i++
			closed := false
			for i < len(query) {
				c := query[i]
				i++
				if c == '\\' || c == 0 {
					return model.Fail("query_denied", "escaped cloud SQL literals are not supported")
				}
				if c == ch {
					if i < len(query) && query[i] == ch {
						// Doubled single quotes are portable. Other quoting rules vary.
						if ch != '\'' {
							return model.Fail("query_denied", "escaped identifiers are not supported")
						}
						out.WriteString("''")
						i++
						continue
					}
					out.WriteByte(quote)
					closed = true
					break
				}
				if ch == '`' && c == '"' {
					out.WriteByte('"')
				}
				out.WriteByte(c)
			}
			if !closed {
				return model.Fail("invalid_query", "unterminated SQL quote")
			}
			continue
		}
		if ch == ':' && i+1 < len(query) && query[i+1] == ':' {
			out.WriteString("::")
			i += 2
			continue
		}
		if ch == '?' && kind == "snowflake" || ch == ':' && kind == "databricks" || ch == '@' && kind == "bigquery" {
			i++
			if ch != '?' {
				start := i
				for i < len(query) && (query[i] >= 'a' && query[i] <= 'z' || query[i] >= 'A' && query[i] <= 'Z' || query[i] >= '0' && query[i] <= '9' || query[i] == '_') {
					i++
				}
				if i == start {
					return model.Fail("query_denied", "invalid native parameter marker")
				}
			}
			parameter++
			out.WriteByte('$')
			out.WriteString(strconv.Itoa(parameter))
			continue
		}
		out.WriteByte(ch)
		i++
	}
	normalized := out.String()
	tokens, err := lex(normalized)
	if err != nil {
		return model.Fail("invalid_query", "Invalid cloud SQL")
	}
	for i := 1; i+1 < len(tokens); i++ {
		if tokens[i-1].text == "." && tokens[i+1].text == "(" {
			return model.Fail("query_denied", "Qualified cloud functions are not supported")
		}
	}
	return guardSQL("postgres", normalized)
}
