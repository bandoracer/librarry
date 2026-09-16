package auth

import (
	"context"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestConfigurationCommitAndRollback(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(NewStore(db), nil)
	ctx := context.Background()
	if err := service.SaveConfig(ctx, MethodForms, "original", "original-password"); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, "original", "original-password", true)
	if err != nil {
		t.Fatal(err)
	}
	// Force the second write to fail after the credential update was attempted.
	_, err = db.Exec(`alter table compat_resources add constraint reject_basic check (resource_type <> 'auth-config' or payload->>'method' <> 'basic')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveConfig(ctx, MethodBasic, "changed", "changed-password"); err == nil {
		t.Fatal("expected transaction failure")
	}
	if service.Method() != MethodForms {
		t.Fatal("enforcement changed despite failed save")
	}
	if _, ok := service.VerifyPassword(ctx, "original", "original-password"); !ok {
		t.Fatal("credentials were not rolled back")
	}
	if _, ok := service.ValidateSession(ctx, login.Token); !ok {
		t.Fatal("session revocation was not rolled back")
	}
	var method string
	if err := db.QueryRow(`select payload->>'method' from compat_resources where resource_type='auth-config'`).Scan(&method); err != nil || method != MethodForms {
		t.Fatalf("persisted mode: %q %v", method, err)
	}
	if _, err := db.Exec(`alter table compat_resources drop constraint reject_basic`); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveConfig(ctx, MethodBasic, "changed", "changed-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.ValidateSession(ctx, login.Token); ok {
		t.Fatal("old session survived credential change")
	}
	if _, ok := service.VerifyPassword(ctx, "changed", "changed-password"); !ok {
		t.Fatal("new credentials not committed")
	}
	if err := service.SaveConfig(ctx, MethodForms, "renamed", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.VerifyPassword(ctx, "renamed", "changed-password"); !ok {
		t.Fatal("blank password did not preserve hash")
	}
}

func TestSessionCannotAuthenticateDifferentUser(t *testing.T) {
	service, store := newTestService(t)
	login, err := service.Login(context.Background(), "ryan", "correct horse", false)
	if err != nil {
		t.Fatal(err)
	}
	store.user.ID = "different-user"
	if _, ok := service.ValidateSession(context.Background(), login.Token); ok {
		t.Fatal("session authenticated a different user")
	}
}

func TestStaleLoginCannotCreateSessionAfterPasswordChange(t *testing.T) {
	db := testdb.Open(t)
	store := NewStore(db)
	service := NewService(store, nil)
	ctx := context.Background()
	if err := service.SaveConfig(ctx, MethodForms, "fixture", "old-password"); err != nil {
		t.Fatal(err)
	}
	verified, ok := service.VerifyPassword(ctx, "fixture", "old-password")
	if !ok {
		t.Fatal("could not verify fixture")
	}
	if err := service.SaveConfig(ctx, MethodForms, "fixture", "new-password"); err != nil {
		t.Fatal(err)
	}
	err := store.CreateSession(ctx, Session{TokenHash: "stale", UserID: verified.ID, CredentialHash: verified.PasswordHash, ExpiresAt: service.now().Add(SessionTTL)})
	if err != ErrInvalidCredentials {
		t.Fatalf("stale login was not rejected: %v", err)
	}
}

func TestStartupCredentialReplacementRevokesSessions(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(NewStore(db), nil)
	ctx := context.Background()
	if err := service.EnsureUser(ctx, "fixture", "initial-password"); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, "fixture", "initial-password", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnsureUser(ctx, "fixture", "replacement-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.ValidateSession(ctx, login.Token); ok {
		t.Fatal("old session survived startup credential replacement")
	}
}
