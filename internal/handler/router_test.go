package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMissingAssetDoesNotUseSPAFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/missing-old-chunk.js", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html") {
		t.Fatalf("missing asset must not return SPA HTML fallback: %s", rec.Body.String())
	}
}
