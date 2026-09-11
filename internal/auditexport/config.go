package auditexport

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"maps"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

type Config struct {
	Enabled     bool              `json:"enabled"`
	Protocol    string            `json:"protocol"`
	Endpoint    string            `json:"endpoint"`
	ServiceName string            `json:"service_name"`
	Headers     map[string]string `json:"headers,omitempty"`
	CAPEM       string            `json:"ca_pem"`
	Revision    int64             `json:"revision"`
}

type Update struct {
	Config
	ClearHeaders bool `json:"clear_headers"`
}

var headerName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)

func normalize(c Config) (Config, error) {
	c.Headers = maps.Clone(c.Headers)
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.ServiceName = strings.TrimSpace(c.ServiceName)
	if c.ServiceName == "" {
		c.ServiceName = "mcpdbhub"
	}
	if c.Protocol == "" {
		c.Protocol = "http/protobuf"
	}
	if c.Protocol != "http/protobuf" && c.Protocol != "grpc" {
		return c, errors.New("Choose http/protobuf or grpc.")
	}
	if len(c.ServiceName) > 128 || strings.ContainsFunc(c.ServiceName, unicode.IsControl) {
		return c, errors.New("Service name must be at most 128 bytes without control characters.")
	}
	if len(c.CAPEM) > 256<<10 {
		return c, errors.New("CA certificate exceeds 256 KiB.")
	}
	if c.Endpoint != "" || c.Enabled {
		u, err := url.Parse(c.Endpoint)
		if err != nil || len(c.Endpoint) > 2048 || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return c, errors.New("Endpoint must be an HTTP(S) URL without credentials, query parameters or fragments.")
		}
		if c.Protocol == "grpc" {
			if (u.Path != "" && u.Path != "/") || u.Port() == "" {
				return c, errors.New("gRPC endpoint must include a port and have no path.")
			}
			u.Path = ""
		} else if u.Path == "" || u.Path == "/" {
			u.Path = "/v1/logs"
		}
		if c.CAPEM != "" && u.Scheme != "https" {
			return c, errors.New("A custom CA requires an HTTPS endpoint.")
		}
		c.Endpoint = u.String()
	}
	if _, err := tlsConfig(c.CAPEM); err != nil {
		return c, err
	}
	if len(c.Headers) > 16 {
		return c, errors.New("At most 16 export headers are allowed.")
	}
	headers := map[string]string{}
	total := 0
	for key, value := range c.Headers {
		key = strings.ToLower(key)
		if !headerName.MatchString(key) || len(key) > 128 || strings.HasPrefix(key, "grpc-") || strings.HasPrefix(key, "content-") || strings.HasSuffix(key, "-bin") {
			return c, errors.New("Invalid or reserved export header name.")
		}
		switch key {
		case "host", "connection", "te", "trailer", "transfer-encoding", "accept-encoding", "user-agent":
			return c, errors.New("Reserved export header name.")
		}
		if _, exists := headers[key]; exists {
			return c, errors.New("Export header names must be unique, ignoring case.")
		}
		for _, ch := range value {
			if ch < 32 || ch > 126 {
				return c, errors.New("Export header values must use printable ASCII.")
			}
		}
		total += len(key) + len(value)
		if total > 16<<10 {
			return c, errors.New("Export headers exceed 16 KiB.")
		}
		headers[key] = value
	}
	c.Headers = headers
	return c, nil
}

func tlsConfig(pem string) (*tls.Config, error) {
	c := &tls.Config{MinVersion: tls.VersionTLS12}
	if pem != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(pem)) {
			return nil, errors.New("CA certificate must contain valid PEM certificates.")
		}
		c.RootCAs = roots
	}
	return c, nil
}
