package adapter

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/SamuelSupe/contextGate/internal/model"
	"golang.org/x/oauth2/jwt"
)

func CloudSQL(kind string) bool {
	return kind == "snowflake" || kind == "databricks" || kind == "bigquery"
}

func cloudOptions(kind string) map[string]bool {
	switch kind {
	case "snowflake":
		return wordset("warehouse schema role token_type read_only_confirmed")
	case "databricks":
		return wordset("warehouse_id schema read_only_confirmed")
	case "bigquery":
		return wordset("schema location maximum_bytes_billed read_only_confirmed")
	}
	return nil
}

var cloudID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var bigQueryProject = regexp.MustCompile(`^[a-z][a-z0-9-]{4,61}[a-z0-9]$`)

func validateCloud(s *model.Source) error {
	if s.TLSMode != "verify" || s.Port != 443 {
		return errors.New("cloud SQL APIs require HTTPS with certificate verification on port 443")
	}
	if s.Options["read_only_confirmed"] != "true" {
		return errors.New("confirm that the cloud credential uses a dedicated read-only data role")
	}
	if s.Database == "" {
		return errors.New("a database, catalog or billing project is required")
	}
	if s.Version == "" {
		s.Version = "1"
	}
	if len(s.Version) > 120 || strings.ContainsAny(s.Version, "\x00\r\n") {
		return errors.New("invalid cloud connection contract version")
	}
	if s.Kind == "bigquery" {
		if s.Host != "bigquery.googleapis.com" || !bigQueryProject.MatchString(s.Database) {
			return errors.New("BigQuery requires bigquery.googleapis.com and a valid project ID")
		}
		if !cloudID.MatchString(s.Options["location"]) {
			return errors.New("BigQuery location is required, for example US or europe-west1")
		}
		if s.Options["maximum_bytes_billed"] == "" {
			s.Options["maximum_bytes_billed"] = "1073741824"
		}
		if n, err := strconv.ParseInt(s.Options["maximum_bytes_billed"], 10, 64); err != nil || n <= 0 {
			return errors.New("maximum_bytes_billed must be a positive int64 decimal string")
		}
		if s.Options["schema"] != "" && !cloudID.MatchString(s.Options["schema"]) {
			return errors.New("invalid BigQuery default dataset")
		}
		if s.AuthMode == "service_account" {
			_, err := bigQueryCredentials(s.Password)
			return err
		}
	}
	if s.Kind == "snowflake" {
		if s.Options["warehouse"] == "" || s.Options["role"] == "" {
			return errors.New("Snowflake warehouse and dedicated reader role are required")
		}
		if v := s.Options["token_type"]; v != "" && v != "OAUTH" && v != "PROGRAMMATIC_ACCESS_TOKEN" {
			return errors.New("Snowflake token_type must be OAUTH or PROGRAMMATIC_ACCESS_TOKEN")
		}
	}
	if s.Kind == "databricks" && !cloudID.MatchString(s.Options["warehouse_id"]) {
		return errors.New("a valid Databricks SQL warehouse ID is required")
	}
	if s.Token == "" || s.Password != "" || s.Username != "" || (s.AuthMode != "" && s.AuthMode != "token") {
		return errors.New("this cloud SQL source requires a bearer access token")
	}
	s.AuthMode = "token"
	return nil
}

func bigQueryCredentials(raw string) (*jwt.Config, error) {
	var key struct {
		Type         string `json:"type"`
		Email        string `json:"client_email"`
		PrivateKey   string `json:"private_key"`
		PrivateKeyID string `json:"private_key_id"`
		TokenURI     string `json:"token_uri"`
	}
	if len(raw) > 64<<10 || json.Unmarshal([]byte(raw), &key) != nil || key.Type != "service_account" || key.Email == "" || !strings.Contains(key.PrivateKey, "PRIVATE KEY") || (key.TokenURI != "" && key.TokenURI != "https://oauth2.googleapis.com/token") {
		return nil, errors.New("invalid Google service account JSON; only the Google OAuth token endpoint is allowed")
	}
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return nil, errors.New("invalid Google service account private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid Google service account PKCS8 private key")
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok || rsaKey.N.BitLen() < 2048 {
		return nil, errors.New("Google service account requires an RSA key of at least 2048 bits")
	}
	return &jwt.Config{Email: key.Email, PrivateKey: []byte(key.PrivateKey), PrivateKeyID: key.PrivateKeyID, Scopes: []string{"https://www.googleapis.com/auth/bigquery"}, TokenURL: "https://oauth2.googleapis.com/token"}, nil
}
