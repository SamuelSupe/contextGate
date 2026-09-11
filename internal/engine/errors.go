package engine

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/gocql/gocql"
	"github.com/mattn/go-sqlite3"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"net"
	"strings"
)

// Only stable codes cross the database boundary. Driver messages may contain
// credentials, query text, parameters, file paths or server addresses.
func PublicError(err error) *model.Error {
	var known *model.Error
	if errors.As(err, &known) {
		copy := *known
		return &copy
	}
	code, native := "database_error", ""
	var certificate *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var dns *net.DNSError
	var network net.Error
	var state interface{ SQLState() string }
	var my *mysql.MySQLError
	var sqlite sqlite3.Error
	var mongoErr mongo.CommandError
	var cql gocql.RequestError
	var graph *neo4j.Neo4jError
	var graphConnection *neo4j.ConnectivityError
	var column *clickhouse.Exception
	var file *duckdb.Error
	var keyValue redis.Error
	switch {
	case errors.Is(err, context.Canceled):
		code = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.As(err, &certificate), errors.As(err, &unknownCA), errors.As(err, &hostname), errors.As(err, &invalidCert):
		code = "database_tls"
	case errors.As(err, &dns):
		code = "database_dns"
	case errors.As(err, &state):
		native = state.SQLState()
		switch {
		case strings.HasPrefix(native, "28"):
			code = "database_authentication"
		case native == "42501":
			code = "database_permission"
		case strings.HasPrefix(native, "42"):
			code = "database_query"
		case strings.HasPrefix(native, "08"), strings.HasPrefix(native, "3D"):
			code = "database_connection"
		}
	case errors.As(err, &my):
		native = fmt.Sprint(my.Number)
		switch my.Number {
		case 1045, 1698:
			code = "database_authentication"
		case 1044, 1142, 1143, 1227:
			code = "database_permission"
		case 1049, 2002, 2003, 2006, 2013:
			code = "database_connection"
		case 1054, 1064, 1146:
			code = "database_query"
		}
	case errors.As(err, &mongoErr):
		native = fmt.Sprint(mongoErr.Code)
		switch mongoErr.Code {
		case 18:
			code = "database_authentication"
		case 13:
			code = "database_permission"
		case 2, 9, 14, 16872:
			code = "database_query"
		}
	case errors.As(err, &sqlite):
		native = fmt.Sprint(int(sqlite.ExtendedCode))
		switch sqlite.Code {
		case sqlite3.ErrPerm, sqlite3.ErrAuth, sqlite3.ErrReadonly:
			code = "database_permission"
		case sqlite3.ErrCantOpen:
			code = "database_connection"
		case sqlite3.ErrError:
			code = "database_query"
		}
	case errors.As(err, &cql):
		native = fmt.Sprintf("0x%04x", cql.Code())
		switch cql.Code() {
		case gocql.ErrCodeCredentials:
			code = "database_authentication"
		case gocql.ErrCodeUnauthorized:
			code = "database_permission"
		case gocql.ErrCodeSyntax, gocql.ErrCodeInvalid, gocql.ErrCodeConfig:
			code = "database_query"
		case gocql.ErrCodeUnavailable:
			code = "database_connection"
		case gocql.ErrCodeReadTimeout:
			code = "timeout"
		}
	case errors.As(err, &graph):
		native = graph.Code
		switch {
		case strings.HasSuffix(native, "Security.Unauthorized"), strings.HasSuffix(native, "Security.CredentialsExpired"):
			code = "database_authentication"
		case strings.HasPrefix(native, "Neo.ClientError.Security."):
			code = "database_permission"
		case strings.HasPrefix(native, "Neo.ClientError.Statement."):
			code = "database_query"
		}
	case errors.As(err, &graphConnection):
		code = "database_connection"
	case errors.As(err, &column):
		native = fmt.Sprint(column.Code)
		switch column.Code {
		case 516:
			code = "database_authentication"
		case 497:
			code = "database_permission"
		case 62, 47, 60:
			code = "database_query"
		}
	case errors.As(err, &file):
		native = fmt.Sprint(file.Type)
		switch file.Type {
		case duckdb.ErrorTypePermission:
			code = "database_permission"
		case duckdb.ErrorTypeConnection, duckdb.ErrorTypeNetwork, duckdb.ErrorTypeIO:
			code = "database_connection"
		case duckdb.ErrorTypeParser, duckdb.ErrorTypeSyntax, duckdb.ErrorTypeBinder, duckdb.ErrorTypeCatalog, duckdb.ErrorTypeConversion:
			code = "database_query"
		}
	case errors.As(err, &keyValue):
		prefix, _, _ := strings.Cut(keyValue.Error(), " ")
		switch prefix {
		case "NOAUTH", "WRONGPASS":
			code = "database_authentication"
			native = prefix
		case "NOPERM":
			code = "database_permission"
			native = prefix
		case "WRONGTYPE", "CROSSSLOT":
			code = "database_query"
			native = prefix
		case "LOADING", "CLUSTERDOWN", "MASTERDOWN":
			code = "database_connection"
			native = prefix
		}
	case errors.As(err, &network):
		code = "database_connection"
		if network.Timeout() {
			code = "timeout"
		}
	}
	return &model.Error{Code: code, Message: ErrorMessage(code), NativeCode: native}
}

func ErrorMessage(code string) string {
	switch code {
	case "cancelled":
		return "Query cancelled or authorization changed."
	case "timeout":
		return "The operation timed out. Check connectivity or reduce the query scope."
	case "database_tls":
		return "TLS verification failed. Check the CA certificate, hostname and certificate expiry."
	case "database_dns":
		return "The database hostname could not be resolved. Check DNS from the Hub server."
	case "database_connection":
		return "The database could not be reached or opened. Check the host, port, database name and server availability."
	case "database_authentication":
		return "Database authentication failed. Check the authentication method and update the credential."
	case "database_permission":
		return "The database denied access. Check the configured account's read permissions."
	case "database_query":
		return "The database rejected the query or object name. Check the dialect, parameters and selected namespace."
	default:
		return "The database operation failed. Use the request ID to locate the audit record and check the database server logs."
	}
}
