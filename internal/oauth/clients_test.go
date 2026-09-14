package oauth

import (
	"context"
	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
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

type stalledBody struct {
	reading chan struct{}
	release chan struct{}
}

func (b *stalledBody) Read([]byte) (int, error) {
	close(b.reading)
	<-b.release
	return 0, io.ErrUnexpectedEOF
}
func (b *stalledBody) Close() error { return nil }

func TestSlowRequestBodyDoesNotBlockOtherOAuthClients(t *testing.T) {
	st, err := store.Open(t.TempDir(), testpg.DSN(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	en := engine.New(st)
	defer en.Close()
	server := New(st, en, "http://127.0.0.1:8080")
	for name, test := range map[string]struct {
		handler     http.HandlerFunc
		contentType string
	}{
		"token":            {server.Token, "application/x-www-form-urlencoded"},
		"revoke":           {server.Revoke, "application/x-www-form-urlencoded"},
		"revoke_multipart": {server.Revoke, "multipart/form-data; boundary=review-boundary"},
		"update_client":    {server.UpdateClient, "application/json"},
		"rotate_secret":    {server.RotateClientSecret, "application/json"},
		"delete_client":    {server.DeleteClient, "application/json"},
	} {
		t.Run(name, func(t *testing.T) {
			body := &stalledBody{reading: make(chan struct{}), release: make(chan struct{})}
			unblock := sync.OnceFunc(func() { close(body.release) })
			request := httptest.NewRequest("POST", "/", body)
			request.Header.Set("Content-Type", test.contentType)
			finished := make(chan struct{})
			go func() { test.handler(httptest.NewRecorder(), request); close(finished) }()
			defer func() { unblock(); <-finished }()
			select {
			case <-body.reading:
			case <-time.After(time.Second):
				t.Fatal("handler did not read the request body")
			}
			registered := make(chan error, 1)
			registrationDone := make(chan struct{})
			go func() {
				defer close(registrationDone)
				_, err := server.Register(context.Background(), Registration{Name: "Independent client", RedirectURIs: []string{"http://127.0.0.1:4444/callback"}}, false)
				registered <- err
			}()
			defer func() { unblock(); <-registrationDone }()
			select {
			case err := <-registered:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Error("an incomplete request body blocked an independent OAuth client")
			}
		})
	}
}

func TestConsentUsesStoredFormWithoutReadingBodyAgain(t *testing.T) {
	st, err := store.Open(t.TempDir(), testpg.DSN(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	en := engine.New(st)
	defer en.Close()
	server := New(st, en, "http://127.0.0.1:8080")
	if err := server.storage.put(context.Background(), "consent", "pending", url.Values{"resource": {server.Resource}}, time.Now().Add(time.Minute), ""); err != nil {
		t.Fatal(err)
	}
	tail := &stalledBody{reading: make(chan struct{}), release: make(chan struct{})}
	request := httptest.NewRequest("POST", "/api/oauth/consent", io.MultiReader(strings.NewReader(`{"request":"pending","allow":false}`), tail))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=review-boundary")
	finished := make(chan struct{})
	go func() { server.Consent(httptest.NewRecorder(), request); close(finished) }()
	defer func() { close(tail.release); <-finished }()
	select {
	case <-finished:
	case <-tail.reading:
		t.Error("consent reparsed the submitted body instead of using the stored authorization form")
	case <-time.After(time.Second):
		t.Error("consent did not finish")
	}
}

func TestDisabledMetadataClientCannotRegisterOverAdministratorState(t *testing.T) {
	st, err := store.Open(t.TempDir(), testpg.DSN(t, t.TempDir()))
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
