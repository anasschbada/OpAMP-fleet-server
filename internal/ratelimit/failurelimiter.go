// Package ratelimit provides a small per-key failed-attempt throttle, used
// to slow down credential-guessing against the shared bearer tokens (see
// internal/auth) without needing an external rate-limiting layer.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// FailureLimiter tracks failed attempts per key (typically a source IP)
// within a sliding window and blocks further attempts once a threshold is
// reached, until the window rolls over. It does not limit successful
// traffic -- only repeated failures, which is what credential guessing
// looks like.
type FailureLimiter struct {
	max    int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count      int
	windowFrom time.Time
	lastSeen   time.Time
}

func NewFailureLimiter(max int, window time.Duration) *FailureLimiter {
	return &FailureLimiter{max: max, window: window, buckets: make(map[string]*bucket)}
}

// Allow reports whether an attempt from key may proceed right now (i.e.
// this key has not exceeded max failures within the current window).
// Call this BEFORE checking credentials, so a key already over the limit
// never even reaches the (comparatively expensive, and log-worthy)
// verification step.
func (l *FailureLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		return true
	}
	if time.Since(b.windowFrom) > l.window {
		// Window elapsed: this key gets a clean slate rather than being
		// evaluated against a stale count.
		return true
	}
	return b.count < l.max
}

// RecordFailure counts one failed attempt from key against its window.
func (l *FailureLimiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.windowFrom) > l.window {
		b = &bucket{windowFrom: now}
		l.buckets[key] = b
	}
	b.count++
	b.lastSeen = now
}

// RecordSuccess clears key's failure count, so a legitimate caller who
// mistyped a credential a few times isn't left throttled after getting it
// right.
func (l *FailureLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// StartCleanup periodically evicts buckets that have been idle well beyond
// the window, so tracking many distinct source IPs over time doesn't grow
// this map unbounded. Blocks until ctx is cancelled; run in its own
// goroutine.
func (l *FailureLimiter) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.evictStale()
			}
		}
	}()
}

func (l *FailureLimiter) evictStale() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-10 * l.window)
	for key, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
