package main

import (
	"bytes"
	"github.com/SamuelSupe/mcpdbhub/internal/model"
	"github.com/SamuelSupe/mcpdbhub/internal/secure"
	"github.com/SamuelSupe/mcpdbhub/internal/store"
	"strings"
	"testing"
	"time"
)

func TestPasswordRecoveryPreservesConfigurationAndRevokesSessions(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Setup(secure.Password("original-password")); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveSource(model.Source{ID: "kept", Name: "Kept source", Password: "database-secret"}); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveAgent(model.Agent{ID: "reader", Enabled: true, Sources: []string{"kept"}}, "kept-token"); err != nil {
		t.Fatal(err)
	}
	if err = st.Session("session", "csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	st.Close()
	var output bytes.Buffer
	password := "replacement-password-123"
	if err = resetPassword([]string{"--data-dir", dir, "--password-stdin"}, strings.NewReader(password+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), password) {
		t.Fatal("password leaked to output")
	}
	st, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	hash, _ := st.Get("admin_password")
	if !secure.CheckPassword(hash, password) || secure.CheckPassword(hash, "original-password") {
		t.Fatal("password was not replaced")
	}
	if _, err = st.CheckSession("session"); err == nil {
		t.Fatal("old administrator session retained")
	}
	if _, err = st.TokenAgent("kept-token"); err != nil {
		t.Fatal("Agent credential lost", err)
	}
	src, err := st.Source("kept")
	if err != nil || src.Password != "database-secret" {
		t.Fatal("data source credential lost", err)
	}
	if err = resetPassword([]string{"--data-dir", dir, "--password-stdin"}, strings.NewReader("short"), &output); err == nil {
		t.Fatal("weak recovery password accepted")
	}
	hashAfter, _ := st.Get("admin_password")
	if hash != hashAfter {
		t.Fatal("failed recovery modified password")
	}
}
