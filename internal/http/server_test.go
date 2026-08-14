package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
)

func TestNewRouter_RoutesAreRegistered(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	router := NewRouter(testAllowedOrigins, testMaxRequestBodyBytes, noopHealthCheck, allow, allow, allow, createSvc, getSvc)

	srv := httptest.NewServer(router)
	defer srv.Close()

	getResp, err := http.Get(srv.URL + "/url/0000000")
	if err != nil {
		t.Fatalf("GET /url/{id} request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected GET /url/{id} to be routed (404 for unknown id), got %d", getResp.StatusCode)
	}
}

func TestNewRouter_CORSHeadersPresent(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	router := NewRouter([]string{"https://example.com"}, testMaxRequestBodyBytes, noopHealthCheck, allow, allow, allow, createSvc, getSvc)

	req := httptest.NewRequest(http.MethodOptions, "/url", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin to be echoed back, got %q", got)
	}
}

func TestHealthz_Healthy(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	router := NewRouter(testAllowedOrigins, testMaxRequestBodyBytes, noopHealthCheck, allow, allow, allow, createSvc, getSvc)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHealthz_Unhealthy(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	failingHealthCheck := func(context.Context) error { return errors.New("redis unreachable") }
	router := NewRouter(testAllowedOrigins, testMaxRequestBodyBytes, failingHealthCheck, allow, allow, allow, createSvc, getSvc)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}
