package main

import (
	"context"
	"errors"
	"flag"
	"github.com/SamuelSupe/mcpdbhub/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copy)
}
func stdio(args []string) error {
	f := flag.NewFlagSet("stdio", flag.ContinueOnError)
	endpoint := f.String("url", env("MCPDBHUB_URL", "http://127.0.0.1:8080/mcp"), "MCP HTTP endpoint")
	if e := f.Parse(args); e != nil {
		return e
	}
	token := os.Getenv("MCPDBHUB_TOKEN")
	if token == "" {
		return errors.New("MCPDBHUB_TOKEN is required")
	}
	u, e := url.Parse(*endpoint)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid MCP endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || !(u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())) {
		return errors.New("remote MCP endpoint requires HTTPS")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpdbhub-stdio", Version: version.Version}, nil)
	session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: *endpoint, HTTPClient: &http.Client{Transport: &bearerTransport{token, http.DefaultTransport}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("MCP redirects are denied") }}}, nil)
	if e != nil {
		return e
	}
	defer session.Close()
	tools, e := session.ListTools(ctx, &mcp.ListToolsParams{})
	if e != nil {
		return e
	}
	bridge := mcp.NewServer(&mcp.Implementation{Name: "mcpdbhub", Version: version.Version}, nil)
	for _, tool := range tools.Tools {
		tool := tool
		bridge.AddTool(tool, func(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return session.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: r.Params.Arguments})
		})
	}
	return bridge.Run(ctx, &mcp.StdioTransport{})
}
