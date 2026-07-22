// Package auth provides shared-bearer-token verification for both the OpAMP
// endpoint (agents) and the REST API (UI/operators). There is no user
// database and no session state: every request/connection carries an
// "Authorization: Bearer <token>" header, checked against a set of tokens
// mounted from a Kubernetes Secret. Supporting multiple valid tokens at once
// allows rotation without a synchronized restart of every collector.
package auth

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// TokenVerifier holds the current set of accepted tokens, as SHA-256
// digests -- never the raw token bytes -- so a memory dump or an accidental
// log of the internal struct cannot leak a credential.
type TokenVerifier struct {
	path string

	mu     sync.RWMutex
	hashes [][sha256.Size]byte
}

// NewTokenVerifier loads tokens from path (one per line; blank lines and
// lines starting with '#' are ignored) and returns an error if the file is
// missing, unreadable, or contains no usable tokens.
func NewTokenVerifier(path string) (*TokenVerifier, error) {
	v := &TokenVerifier{path: path}
	if err := v.Reload(); err != nil {
		return nil, err
	}
	return v, nil
}

// Reload re-reads the token file. Call it periodically (see
// StartAutoReload) so rotating the mounted Secret takes effect without
// restarting the server pod.
func (v *TokenVerifier) Reload() error {
	f, err := os.Open(v.path)
	if err != nil {
		return fmt.Errorf("open auth tokens file %s: %w", v.path, err)
	}
	defer f.Close()

	var hashes [][sha256.Size]byte
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hashes = append(hashes, sha256.Sum256([]byte(line)))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read auth tokens file %s: %w", v.path, err)
	}
	if len(hashes) == 0 {
		return fmt.Errorf("auth tokens file %s contains no tokens", v.path)
	}

	v.mu.Lock()
	v.hashes = hashes
	v.mu.Unlock()
	return nil
}

// Verify reports whether token matches one of the currently loaded tokens.
// The comparison is constant-time and does not short-circuit on the first
// match, so neither the total token count nor which entry matched is
// observable via timing.
func (v *TokenVerifier) Verify(token string) bool {
	if token == "" {
		return false
	}
	sum := sha256.Sum256([]byte(token))

	v.mu.RLock()
	defer v.mu.RUnlock()

	matched := 0
	for _, h := range v.hashes {
		matched |= subtle.ConstantTimeCompare(sum[:], h[:])
	}
	return matched == 1
}

// StartAutoReload reloads the token file every interval until ctx is
// cancelled. Reload failures (e.g. the file briefly missing during a
// Secret update) are logged and the previously loaded tokens keep being
// accepted -- a transient read error must never lock out every agent.
func (v *TokenVerifier) StartAutoReload(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := v.Reload(); err != nil {
					logger.Warn("auth token reload failed, keeping previous tokens", "error", err)
				}
			}
		}
	}()
}

// BearerToken extracts the token from a standard "Authorization: Bearer
// <token>" header value. Returns "" if the header is absent or malformed.
func BearerToken(headerValue string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(headerValue, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(headerValue, prefix))
}
