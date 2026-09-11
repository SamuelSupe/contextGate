package oauth

import (
	"context"
	"github.com/SamuelSupe/mcpdbhub/internal/engine"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetadataFetchCannotReachInternalNetworks(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "64:ff9b::a00:1", "198.18.0.1"} {
		if publicIP(net.ParseIP(ip)) {
			t.Errorf("internal address accepted: %s", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicIP(net.ParseIP(ip)) {
			t.Errorf("public address rejected: %s", ip)
		}
	}
	if _, err := fetchMetadata(context.Background(), "https://127.0.0.1:9/client.json"); err == nil {
		t.Fatal("private metadata URL accepted")
	}
	for _, uri := range []string{"https://app.example/callback#fragment", "http://app.example/callback", "https://user:pass@app.example/callback", "javascript:alert(1)"} {
		if validateRedirect(uri) == nil {
			t.Errorf("unsafe callback accepted: %s", uri)
		}
	}
}

func TestDisabledMetadataClientCannotRegisterOverAdministratorState(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	en := engine.New(st)
	defer en.Close()
	server := New(st, en, "http://127.0.0.1:8080")
	input := Registration{ClientID: "https://client.example/metadata.json", Name: "Metadata client", RedirectURIs: []string{"http://127.0.0.1:4444/callback"}}
	if _, err := server.Register(context.Background(), input, true); err != nil {
		t.Fatal(err)
	}
	var client Client
	server.storage.get(context.Background(), "client", input.ClientID, &client)
	request := httptest.NewRequest("PUT", "/api/oauth/clients/metadata", strings.NewReader(`{"revision":"1","client_name":"Metadata client","redirect_uris":["http://127.0.0.1:4444/callback"],"enabled":false}`))
	request.SetPathValue("id", input.ClientID)
	response := httptest.NewRecorder()
	server.UpdateClient(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if _, err := server.Register(context.Background(), input, true); err == nil {
		t.Fatal("metadata refresh bypassed client disable")
	}
	if _, err := server.storage.GetClient(context.Background(), input.ClientID); err == nil {
		t.Fatal("disabled client remained usable")
	}
}
