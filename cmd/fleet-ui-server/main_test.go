package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

// Regression test for a finding from the security review: a null byte (or
// other control character) in the request path used to reach
// http.FileServer and come back as a bare 500, since os.Open's error for
// an embedded NUL isn't one http.FileServer recognizes as "not found".
// withRejectedControlChars must catch it first with a clean 400.
func TestWithRejectedControlChars(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := withRejectedControlChars(inner)

	cases := []struct {
		path string
		want int
	}{
		{"/index.html", http.StatusOK},
		{"/assets/app.js", http.StatusOK},
		{"/index.html\x00.png", http.StatusBadRequest},
		{"/\x01\x02", http.StatusBadRequest},
		{"/\x7f", http.StatusBadRequest},
	}
	for _, c := range cases {
		// Built by hand rather than httptest.NewRequest(method, url, ...):
		// url.Parse (which that helper uses internally) rejects control
		// characters outright, but a real client CAN still get one to a
		// Go http.Server -- its own request-line parsing is more lenient
		// than url.Parse, which is exactly how this bug was originally
		// found (a live curl request with a URL-encoded null byte reached
		// the handler and came back as a bare 500). Setting r.URL.Path
		// directly reproduces what the handler actually sees in that case.
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.URL = &url.URL{Path: c.path}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("path %q: got %d, want %d", c.path, rec.Code, c.want)
		}
	}
}

func TestWithRecover_PanicBecomes500NotCrash(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	handler := withRecover(log, panicky)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req) // must not panic out of the test itself

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

func TestOriginOf(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"https://api.example.com", "https://api.example.com", false},
		{"https://api.example.com:8443/some/path", "https://api.example.com:8443", false},
		{"not-a-url", "", true},
		{"/relative/path", "", true},
	}
	for _, c := range cases {
		got, err := originOf(c.in)
		if c.wantErr && err == nil {
			t.Errorf("originOf(%q): expected an error", c.in)
		}
		if !c.wantErr && err != nil {
			t.Errorf("originOf(%q): unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("originOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
