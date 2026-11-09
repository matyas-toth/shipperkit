package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
)

func TestProviderSecretSealingAndLastAdminRoutes(t *testing.T) {
	ctx := context.Background()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	password, _ := auth.Password("administrator-password-123")
	if err = db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@example.test", PasswordHash: password, Role: RoleAdmin}, "", "", 0); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveAuthProvider(ctx, "google", "google-client", "super-secret-value", true, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err = db.db.QueryRow("SELECT value FROM settings WHERE key='auth_google_secret'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "sealed:v1:") || strings.Contains(raw, "super-secret-value") {
		t.Fatal("provider secret was stored in readable form")
	}
	provider, err := db.AuthProvider(ctx, "google")
	if err != nil || !provider.Enabled || !provider.HasSecret || provider.ClientID != "google-client" {
		t.Fatalf("provider metadata incorrect: %+v %v", provider, err)
	}
	if secret, err := db.AuthProviderSecret(ctx, "google"); err != nil || secret != "super-secret-value" {
		t.Fatalf("provider secret did not round trip: %q %v", secret, err)
	}
	if err = db.SaveAuthProvider(ctx, "google", "rotated-client", "rotated-secret", true, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	if secret, err := db.AuthProviderSecret(ctx, "google"); err != nil || secret != "rotated-secret" {
		t.Fatalf("provider secret did not rotate: %q %v", secret, err)
	}

	// With only a password route, password login cannot be disabled and the
	// administrator cannot be demoted or stripped of the credential.
	if err = db.SetPasswordLoginEnabled(ctx, false, "owner", time.Now()); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("password route was removed without another method: %v", err)
	}
	if err = db.UpdateUserRole(ctx, "owner", RoleDeveloper, "owner", time.Now()); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last administrator was demoted: %v", err)
	}
	if err = db.RemoveUserPassword(ctx, "owner", "owner", time.Now()); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last administrator lost its password: %v", err)
	}

	// A linked provider identity makes password login removable, but removing
	// that provider then becomes the last-route violation.
	if _, err = db.db.Exec("INSERT INTO user_identities(provider,subject,user_id,email,created_at,last_login_at) VALUES ('google','subject-1','owner','owner@example.test',?,?)", time.Now().Unix(), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if err = db.SetPasswordLoginEnabled(ctx, false, "owner", time.Now()); err != nil {
		t.Fatalf("provider route did not permit disabling password login: %v", err)
	}
	if err = db.ClearAuthProvider(ctx, "google", "owner", time.Now()); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("last provider route was removed: %v", err)
	}
}

func TestDisabledAccountRevokesCredentials(t *testing.T) {
	ctx := context.Background()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	password, _ := auth.Password("administrator-password-123")
	if err = db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@example.test", PasswordHash: password, Role: RoleAdmin}, "", "", 0); err != nil {
		t.Fatal(err)
	}
	member := User{ID: "member", Email: "member@example.test", Role: RoleDeveloper, HasPassword: true}
	if err = db.CreateUser(ctx, member, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	hash := auth.Hash("member-token")
	if err = db.CreateSession(ctx, hash, "member", "csrf", time.Now().Add(time.Hour).Unix(), "password", nil); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveOperationToken(ctx, OperationToken{ID: "token", Name: "test", Hash: auth.Hash("raw-token"), UserID: "member", Scopes: []string{"read"}, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if err = db.SetUserDisabled(ctx, "member", true, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Session(ctx, hash); err == nil {
		t.Fatal("disabled account kept its browser session")
	}
	if _, err = db.OperationToken(ctx, "raw-token"); err == nil {
		t.Fatal("disabled account kept its operations token")
	}
	if _, err = db.FindUser(ctx, "member@example.test"); err == nil {
		t.Fatal("disabled account could sign in with a password")
	}
	if err = db.SetUserDisabled(ctx, "member", false, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	if found, err := db.FindUser(ctx, "member@example.test"); err != nil || found.Disabled {
		t.Fatalf("enabled account unavailable: %+v %v", found, err)
	}
}

func TestIdentityBindingDoesNotMoveBetweenAccounts(t *testing.T) {
	ctx := context.Background()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	password, _ := auth.Password("administrator-password-123")
	if err = db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@example.test", PasswordHash: password, Role: RoleAdmin}, "", "", 0); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateUser(ctx, User{ID: "one", Email: "one@example.test", Role: RoleViewer}, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateUser(ctx, User{ID: "two", Email: "two@example.test", Role: RoleViewer}, "owner", time.Now()); err != nil {
		t.Fatal(err)
	}
	hash := auth.Hash("pending-session")
	if err = db.CreateSession(ctx, hash, "one", "csrf", time.Now().Add(time.Hour).Unix(), "google", &Identity{Provider: "google", Subject: "subject-1", Email: "one@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err = db.FinalizeSessionIdentity(ctx, hash, time.Now()); err != nil {
		t.Fatal(err)
	}
	user, err := db.IdentityUser(ctx, "google", "subject-1")
	if err != nil || user.ID != "one" {
		t.Fatalf("identity did not bind to the pending account: %+v %v", user, err)
	}
	// Bound identities are stable even when the account email changes.
	if _, err = db.db.Exec("UPDATE administrators SET email='renamed@example.test' WHERE id='one'"); err != nil {
		t.Fatal(err)
	}
	if user, err = db.IdentityUser(ctx, "google", "subject-1"); err != nil || user.ID != "one" {
		t.Fatalf("identity moved after an email change: %+v %v", user, err)
	}
	// A different session cannot claim the same subject for another account.
	other := auth.Hash("other-session")
	if err = db.CreateSession(ctx, other, "two", "csrf", time.Now().Add(time.Hour).Unix(), "google", &Identity{Provider: "google", Subject: "subject-1", Email: "two@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err = db.FinalizeSessionIdentity(ctx, other, time.Now()); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("identity collided without an error: %v", err)
	}
}

func TestMigrationUpgradesSingleOwnerDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "encryption.key"), []byte("0123456789abcdef0123456789abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", filepath.Join(dir, "shipper.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "014" {
			break
		}
		body, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = raw.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", entry.Name(), err)
		}
		if _, err = raw.Exec("INSERT INTO schema_migrations VALUES (?,?)", entry.Name(), time.Now().Unix()); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	if _, err = raw.Exec("INSERT INTO administrators(id,email,password_hash,created_at) VALUES ('owner','owner@example.test','hash',?)", now); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,factor_verified) VALUES ('session','owner','csrf',?,1)", now+3600); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("INSERT INTO account_factors(user_id,secret,last_step) VALUES ('owner','fixture',1)"); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("INSERT INTO factor_recovery_codes(user_id,code_hash) VALUES ('owner','code')"); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("INSERT INTO settings(key,value) VALUES ('acme_contact','owner@example.test')"); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	session, err := db.Session(context.Background(), "session")
	if err != nil {
		t.Fatal(err)
	}
	if session.User.ID != "owner" || session.User.Role != RoleAdmin || !session.Factor.Enrolled || !session.Verified {
		t.Fatalf("existing account or factor state changed: %+v", session)
	}
	if session.User.PasswordChangeRequired {
		t.Fatal("existing owner was marked temporary")
	}
	var count int
	if err = db.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version='014_collaboration.sql'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("collaboration migration was not recorded: %d %v", count, err)
	}
}
