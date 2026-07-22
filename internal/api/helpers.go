package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// maxRequestBodyBytes bounds every request body the API reads. Config YAML
// bodies are the largest legitimate payload; 1 MiB is generous for a
// collector config while still refusing an oversized body before it's fully
// buffered in memory (a cheap denial-of-service vector otherwise).
const maxRequestBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Encoding errors here mean the connection is already gone (client
	// disconnected mid-write); there is nothing left to do but ignore it.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, log *slog.Logger, status int, publicMessage string, err error) {
	if err != nil && status >= 500 {
		log.Error("request failed", "status", status, "error", err)
	}
	writeJSON(w, status, map[string]string{"error": publicMessage})
}

// decodeJSON reads and decodes a JSON request body, rejecting bodies over
// maxRequestBodyBytes and any unknown/malformed fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
