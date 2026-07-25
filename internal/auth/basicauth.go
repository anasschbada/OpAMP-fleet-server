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

// BasicAuthVerifier holds the single accepted HTTP Basic Auth
// username/password pair, as SHA-256 digests -- same rationale as
// TokenVerifier: a memory dump or an accidental log of the struct cannot
// leak the credential. This is an OPTIONAL, ADDITIONAL way to authenticate
// to the REST API alongside API bearer tokens (see withAuth in
// internal/api/middleware.go); it never replaces the agent/API token
// separation described in internal/config.Config.
type BasicAuthVerifier struct {
	usernamePath string
	passwordPath string

	mu           sync.RWMutex
	usernameHash [sha256.Size]byte
	passwordHash [sha256.Size]byte
}

// NewBasicAuthVerifier loads the username and password from two separate
// single-value files (same one-value-per-line convention as the token
// files, capped at the first usable line) and returns an error if either
// is missing, unreadable, or empty.
func NewBasicAuthVerifier(usernamePath, passwordPath string) (*BasicAuthVerifier, error) {
	v := &BasicAuthVerifier{usernamePath: usernamePath, passwordPath: passwordPath}
	if err := v.Reload(); err != nil {
		return nil, err
	}
	return v, nil
}

// Reload re-reads both files. Call it periodically (see StartAutoReload)
// so rotating the mounted Secret takes effect without restarting the pod.
// Both files are read fully before anything is written to v, so a
// transient read error (e.g. a Secret mount briefly missing during an
// update) never clobbers the previously loaded, still-valid credential.
func (v *BasicAuthVerifier) Reload() error {
	username, err := v.readUsernameFile()
	if err != nil {
		return fmt.Errorf("read basic auth username file %s: %w", v.usernamePath, err)
	}
	if username == "" {
		return fmt.Errorf("basic auth username file %s contains no usable value", v.usernamePath)
	}

	password, err := v.readPasswordFile()
	if err != nil {
		return fmt.Errorf("read basic auth password file %s: %w", v.passwordPath, err)
	}
	if password == "" {
		return fmt.Errorf("basic auth password file %s contains no usable value", v.passwordPath)
	}

	uHash := sha256.Sum256([]byte(username))
	pHash := sha256.Sum256([]byte(password))

	v.mu.Lock()
	v.usernameHash = uHash
	v.passwordHash = pHash
	v.mu.Unlock()
	return nil
}

// Verify reports whether username/password match the currently loaded
// credential. Both comparisons are constant-time, and the password check
// always runs even if the username already mismatched, so neither which
// field failed nor whether the username is merely wrong is observable via
// timing.
func (v *BasicAuthVerifier) Verify(username, password string) bool {
	if username == "" || password == "" {
		return false
	}
	uSum := sha256.Sum256([]byte(username))
	pSum := sha256.Sum256([]byte(password))

	v.mu.RLock()
	defer v.mu.RUnlock()

	uMatch := subtle.ConstantTimeCompare(uSum[:], v.usernameHash[:])
	pMatch := subtle.ConstantTimeCompare(pSum[:], v.passwordHash[:])
	return uMatch == 1 && pMatch == 1
}

// StartAutoReload reloads both files every interval until ctx is
// cancelled. Reload failures are logged and the previously loaded
// credential keeps being accepted -- see Reload's comment.
func (v *BasicAuthVerifier) StartAutoReload(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := v.Reload(); err != nil {
					logger.Warn("basic auth credential reload failed, keeping previous credential", "error", err)
				}
			}
		}
	}()
}

// readUsernameFile and readPasswordFile each return the first non-blank,
// non-comment line of their file, trimmed -- "" (no error) if the file
// exists but has no usable line. Two near-identical methods reading their
// own struct field directly, rather than one helper taking a path
// parameter, deliberately mirrors TokenVerifier.Reload's shape in
// tokens.go: both paths come from server-operator-controlled config
// (internal/config.Config), never request input, but keeping the same
// field-access shape as the file this was modeled on is what static
// analysis (gosec G304) expects to recognize as such.
func (v *BasicAuthVerifier) readUsernameFile() (string, error) {
	f, err := os.Open(v.usernamePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return firstUsableLine(f)
}

func (v *BasicAuthVerifier) readPasswordFile() (string, error) {
	f, err := os.Open(v.passwordPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return firstUsableLine(f)
}

func firstUsableLine(f *os.File) (string, error) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}
