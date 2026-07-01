package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackendURLRequiresExplicitEnvironment(t *testing.T) {
	t.Setenv("BACKEND_URL", "")
	if got := backendURL(); got != "" {
		t.Fatalf("backendURL() = %q, want empty when BACKEND_URL is not configured", got)
	}

	t.Setenv("BACKEND_URL", " http://backend-1/ ")
	if got := backendURL(); got != "http://backend-1" {
		t.Fatalf("backendURL() = %q, want trimmed explicit value", got)
	}
}

func TestProxyStatusFailsWithoutBackendURL(t *testing.T) {
	t.Setenv("BACKEND_URL", "")
	rec := httptest.NewRecorder()

	proxyStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
