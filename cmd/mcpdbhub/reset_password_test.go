package main

import (
	"bytes"
	"context"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
	"github.com/SamuelSupe/contextGate/internal/testpg"
	"strings"
	"testing"
	"time"
)

func TestPasswordRecoveryPreservesConfigurationAndRevokesSessions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MCPDBHUB_DATABASE_URL", testpg.DSN(t, dir))
	st, err := store.Open(dir, testpg.DSN(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	originalHash := secure.Password("original-password")
	if err = st.Setup(originalHash); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveSource(model.Source{ID: "kept", Name: "Kept source", Password: "database-secret"}); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveAgent(model.Agent{ID: "reader", Enabled: true, Sources: []string{"kept"}}, "kept-token"); err != nil {
		t.Fatal(err)
	}
	if err = st.Session("session", "csrf", time.Now().Add(time.Hour), originalHash); err != nil {
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
	st, err = store.Open(dir, testpg.DSN(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	hash, _ := st.AdministratorByUsername("admin")
	if !secure.CheckPassword(hash.PasswordHash, password) || secure.CheckPassword(hash.PasswordHash, "original-password") {
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
	hashAfter, _ := st.AdministratorByUsername("admin")
	if hash.PasswordHash != hashAfter.PasswordHash {
		t.Fatal("failed recovery modified password")
	}
}

func TestNamedPasswordRecoveryDoesNotRevokeOtherAdministrators(t *testing.T) {
	dir := t.TempDir()
	dsn := testpg.DSN(t, dir)
	t.Setenv("MCPDBHUB_DATABASE_URL", dsn)
	st, err := store.Open(dir, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	firstHash := secure.Password("first-password-123")
	first, err := st.SetupAdministrator("first", "First", firstHash)
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateAdministrator(context.Background(), "second", "Second", model.RoleAdministrator, secure.Password("temporary-password"))
	if err != nil {
		t.Fatal(err)
	}
	second, err = st.ChangeAdministratorPassword(context.Background(), second.ID, second.PasswordHash, secure.Password("second-password-123"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []model.Administrator{first, second} {
		if err = st.CreateAdministratorSession(account.ID, "session-"+account.Username, "csrf", time.Now().Add(time.Hour), account.PasswordHash); err != nil {
			t.Fatal(err)
		}
	}
	_, firstToken, err := st.IssueConfigurationToken(context.Background(), first.ID, "", time.Now().Add(time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, secondToken, err := st.IssueConfigurationToken(context.Background(), second.ID, "", time.Now().Add(time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	replacement := "replacement-second-password"
	if err = resetPassword([]string{"--data-dir", dir, "--password-stdin"}, strings.NewReader(replacement), &output); model.ErrorCode(err) != "invalid_input" {
		t.Fatal("ambiguous recovery selected an account", err)
	}
	if err = resetPassword([]string{"--data-dir", dir, "--username", "SECOND", "--password-stdin"}, strings.NewReader(replacement), &output); err != nil {
		t.Fatal(err)
	}
	recovered, _ := st.Administrator(second.ID)
	unchanged, _ := st.Administrator(first.ID)
	if recovered.Role != model.RoleAdministrator || !secure.CheckPassword(recovered.PasswordHash, replacement) || unchanged.PasswordHash != firstHash {
		t.Fatal("recovery changed an unrelated account or role")
	}
	if _, err = st.CheckSession("session-first"); err != nil {
		t.Fatal("unrelated session revoked", err)
	}
	if _, err = st.CheckSession("session-second"); err == nil {
		t.Fatal("target session survived")
	}
	if _, err = st.ConfigurationToken(firstToken); err != nil {
		t.Fatal("unrelated configuration token revoked", err)
	}
	if _, err = st.ConfigurationToken(secondToken); err == nil {
		t.Fatal("target configuration token survived")
	}
	audit, err := st.Audits(store.AuditFilter{EventKind: "security", Channel: "cli"}, 10)
	if err != nil || len(audit) != 1 || audit[0].ResourceID != second.ID || audit[0].ActorType != "local_operator" || audit[0].ErrorCode != "" {
		t.Fatal("local recovery audit missing", err)
	}
}
