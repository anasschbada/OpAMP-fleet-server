package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func writeValueFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestNewBasicAuthVerifier_ValidCredential(t *testing.T) {
	userPath := writeValueFile(t, "username", "# a comment\n\nadmin\n")
	passPath := writeValueFile(t, "password", "s3cret\n")

	v, err := NewBasicAuthVerifier(userPath, passPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Verify("admin", "s3cret") {
		t.Error("expected the configured credential to verify")
	}
	if v.Verify("admin", "wrong-password") {
		t.Error("wrong password must not verify")
	}
	if v.Verify("wrong-user", "s3cret") {
		t.Error("wrong username must not verify")
	}
	if v.Verify("", "") {
		t.Error("empty credential must never verify")
	}
}

func TestNewBasicAuthVerifier_EmptyUsernameFileRejected(t *testing.T) {
	userPath := writeValueFile(t, "username", "\n# only comments\n\n")
	passPath := writeValueFile(t, "password", "s3cret\n")
	if _, err := NewBasicAuthVerifier(userPath, passPath); err == nil {
		t.Fatal("expected an error for a username file with no usable value")
	}
}

func TestNewBasicAuthVerifier_EmptyPasswordFileRejected(t *testing.T) {
	userPath := writeValueFile(t, "username", "admin\n")
	passPath := writeValueFile(t, "password", "\n")
	if _, err := NewBasicAuthVerifier(userPath, passPath); err == nil {
		t.Fatal("expected an error for a password file with no usable value")
	}
}

func TestNewBasicAuthVerifier_MissingFileRejected(t *testing.T) {
	passPath := writeValueFile(t, "password", "s3cret\n")
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := NewBasicAuthVerifier(missing, passPath); err == nil {
		t.Fatal("expected an error for a missing username file")
	}
}

func TestBasicAuthVerifier_Reload_PicksUpRotation(t *testing.T) {
	userPath := writeValueFile(t, "username", "admin\n")
	passPath := writeValueFile(t, "password", "old-password\n")

	v, err := NewBasicAuthVerifier(userPath, passPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Verify("admin", "old-password") {
		t.Fatal("old-password should verify before rotation")
	}

	if err := os.WriteFile(passPath, []byte("new-password\n"), 0o600); err != nil {
		t.Fatalf("rewrite password file: %v", err)
	}
	if err := v.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if v.Verify("admin", "old-password") {
		t.Error("old-password should no longer verify after rotation")
	}
	if !v.Verify("admin", "new-password") {
		t.Error("new-password should verify after rotation")
	}
}

func TestBasicAuthVerifier_Reload_TransientMissingFileKeepsOldCredential(t *testing.T) {
	userPath := writeValueFile(t, "username", "admin\n")
	passPath := writeValueFile(t, "password", "sticky-password\n")

	v, err := NewBasicAuthVerifier(userPath, passPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Remove(passPath); err != nil {
		t.Fatalf("remove password file: %v", err)
	}
	if err := v.Reload(); err == nil {
		t.Fatal("expected Reload to report the missing file as an error")
	}

	if !v.Verify("admin", "sticky-password") {
		t.Error("a failed reload must not drop the previously loaded credential")
	}
}

func TestBasicAuthVerifier_DifferentLengthInputsDoNotPanic(t *testing.T) {
	userPath := writeValueFile(t, "username", "admin\n")
	passPath := writeValueFile(t, "password", "s3cret\n")
	v, err := NewBasicAuthVerifier(userPath, passPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	long := make([]byte, 10000)
	for i := range long {
		long[i] = 'x'
	}
	if v.Verify(string(long), string(long)) {
		t.Error("unexpected match for unrelated long input")
	}
}
