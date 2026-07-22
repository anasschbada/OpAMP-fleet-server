package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTokenFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.txt")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	return path
}

func TestNewTokenVerifier_ValidTokens(t *testing.T) {
	path := writeTokenFile(t, "token-a\ntoken-b\n# a comment\n\ntoken-c\n")
	v, err := NewTokenVerifier(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, tok := range []string{"token-a", "token-b", "token-c"} {
		if !v.Verify(tok) {
			t.Errorf("expected %q to verify", tok)
		}
	}
	if v.Verify("token-d") {
		t.Error("unexpected token verified")
	}
	if v.Verify("") {
		t.Error("empty token must never verify")
	}
	if v.Verify("# a comment") {
		t.Error("comment lines must not become valid tokens")
	}
}

func TestNewTokenVerifier_EmptyFileRejected(t *testing.T) {
	path := writeTokenFile(t, "\n# only comments\n\n")
	if _, err := NewTokenVerifier(path); err == nil {
		t.Fatal("expected an error for a token file with no usable tokens")
	}
}

func TestNewTokenVerifier_MissingFileRejected(t *testing.T) {
	if _, err := NewTokenVerifier(filepath.Join(t.TempDir(), "does-not-exist.txt")); err == nil {
		t.Fatal("expected an error for a missing token file")
	}
}

func TestTokenVerifier_Reload_PicksUpRotation(t *testing.T) {
	path := writeTokenFile(t, "old-token\n")
	v, err := NewTokenVerifier(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Verify("old-token") {
		t.Fatal("old-token should verify before rotation")
	}

	if err := os.WriteFile(path, []byte("new-token\n"), 0o600); err != nil {
		t.Fatalf("rewrite token file: %v", err)
	}
	if err := v.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if v.Verify("old-token") {
		t.Error("old-token should no longer verify after rotation")
	}
	if !v.Verify("new-token") {
		t.Error("new-token should verify after rotation")
	}
}

func TestTokenVerifier_Reload_TransientMissingFileKeepsOldTokens(t *testing.T) {
	path := writeTokenFile(t, "sticky-token\n")
	v, err := NewTokenVerifier(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove token file: %v", err)
	}
	if err := v.Reload(); err == nil {
		t.Fatal("expected Reload to report the missing file as an error")
	}

	// The whole point of not clobbering v.hashes on a failed reload: a
	// transient Secret-mount hiccup must not lock out every agent.
	if !v.Verify("sticky-token") {
		t.Error("a failed reload must not drop previously loaded tokens")
	}
}

func TestBearerToken(t *testing.T) {
	cases := map[string]string{
		"Bearer abc123":  "abc123",
		"Bearer  abc123": "abc123", // extra space trimmed
		"bearer abc123":  "",       // case-sensitive per RFC 6750
		"abc123":         "",
		"":               "",
		"Bearer ":        "",
	}
	for header, want := range cases {
		if got := BearerToken(header); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestTokenVerifier_DifferentLengthTokensDoNotPanic(t *testing.T) {
	path := writeTokenFile(t, "a-normal-length-token\n")
	v, err := NewTokenVerifier(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Regression guard: comparisons are done on fixed-size SHA-256 sums,
	// never on the raw token bytes, so wildly different input lengths
	// (including empty and very long) must never panic subtle.ConstantTimeCompare.
	longToken := make([]byte, 10000)
	for i := range longToken {
		longToken[i] = 'x'
	}
	if v.Verify(string(longToken)) {
		t.Error("unexpected match for unrelated long input")
	}
}
