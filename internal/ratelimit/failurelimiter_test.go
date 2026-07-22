package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestFailureLimiter_AllowsUntilThreshold(t *testing.T) {
	l := NewFailureLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should still be allowed", i)
		}
		l.RecordFailure("1.2.3.4")
	}
	if l.Allow("1.2.3.4") {
		t.Error("4th attempt should be throttled after 3 failures with max=3")
	}
}

func TestFailureLimiter_KeysAreIndependent(t *testing.T) {
	l := NewFailureLimiter(1, time.Minute)
	l.RecordFailure("1.1.1.1")

	if l.Allow("1.1.1.1") {
		t.Error("1.1.1.1 should be throttled")
	}
	if !l.Allow("2.2.2.2") {
		t.Error("an unrelated key must not be affected by another key's failures")
	}
}

func TestFailureLimiter_SuccessClearsFailures(t *testing.T) {
	l := NewFailureLimiter(1, time.Minute)
	l.RecordFailure("1.2.3.4")
	if l.Allow("1.2.3.4") {
		t.Fatal("should be throttled before success")
	}
	l.RecordSuccess("1.2.3.4")
	if !l.Allow("1.2.3.4") {
		t.Error("a recorded success should clear the failure count")
	}
}

func TestFailureLimiter_WindowRollover(t *testing.T) {
	l := NewFailureLimiter(1, 20*time.Millisecond)
	l.RecordFailure("1.2.3.4")
	if l.Allow("1.2.3.4") {
		t.Fatal("should be throttled immediately after hitting the limit")
	}

	time.Sleep(30 * time.Millisecond)
	if !l.Allow("1.2.3.4") {
		t.Error("should be allowed again once the window has elapsed")
	}
}

func TestFailureLimiter_CleanupEvictsStaleEntries(t *testing.T) {
	l := NewFailureLimiter(1, 10*time.Millisecond)
	l.RecordFailure("1.2.3.4")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.StartCleanup(ctx, 5*time.Millisecond)

	// evictStale removes entries idle for more than 10x the window (see
	// its implementation) -- with a 10ms window that's 100ms, give it a
	// comfortable margin.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		n := len(l.buckets)
		l.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("expected the stale bucket to be evicted within the deadline")
}
