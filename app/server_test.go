package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name, path, contains string
		status               int
	}{
		{"WHEN health is requested THEN status ok is returned", "/healthz", `"status":"ok"`, http.StatusOK},
		{"WHEN the root is requested THEN the index page is served", "/", "<title>", http.StatusOK},
		{"WHEN an unknown path is requested THEN 404 is returned", "/nope", "", http.StatusNotFound},
	}
	h := newServer()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if !strings.Contains(rec.Body.String(), tc.contains) {
				t.Errorf("body %q lacks %q", rec.Body.String(), tc.contains)
			}
		})
	}
}
