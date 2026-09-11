package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/server"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStdioBridgeUsesHTTPAuthorization(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches the stdio executable")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "mcpdbhub")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, out)
	}
	st, e := store.Open(filepath.Join(dir, "config"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	path := filepath.Join(dir, "fixture.db")
	db, _ := sql.Open("sqlite3", path)
	_, e = db.Exec("CREATE TABLE events(id INTEGER);INSERT INTO events VALUES(1),(2)")
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	src := model.Source{ID: "fixture", Name: "fixture", Kind: "sqlite", Path: path, Enabled: true, Revision: 1, TLSMode: "disable"}
	src.Limits.Defaults()
	st.SaveSource(src)
	token := "hub_stdio_fixture_token"
	st.SaveAgent(model.Agent{ID: "reader", Sources: []string{src.ID}, Enabled: true, ExpiresAt: time.Now().Add(time.Hour)}, token)
	ts := httptest.NewUnstartedServer(nil)
	base := "http://" + ts.Listener.Addr().String()
	app, e := server.New(st, base, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	ts.Config.Handler = app.Handler()
	ts.Start()
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	child := exec.Command(binary, "stdio", "--url", base+"/mcp")
	child.Env = append(os.Environ(), "MCPDBHUB_TOKEN="+token)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: child}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "query_sql", Arguments: map[string]any{"source_id": src.ID, "query": "SELECT count(*) AS count FROM events"}})
	if e != nil || r.IsError {
		b, _ := json.Marshal(r)
		t.Fatalf("stdio query: %s %v", b, e)
	}
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `\"count\"`) && !strings.Contains(string(b), `"count"`) {
		t.Fatalf("query data missing from stdio result: %s", b)
	}
	a, _ := st.Agent("reader")
	a.Enabled = false
	st.SaveAgent(a, "")
	app.Engine.InvalidateAgent(a.ID)
	r, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_data_sources", Arguments: map[string]any{}})
	if e == nil && !r.IsError {
		t.Fatal("stdio bridge bypassed HTTP revocation")
	}
}
