package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
)

type fakeLimiter struct {
	allow bool
	err   error
}

func (f *fakeLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return f.allow, f.err
}

func newOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestClientIP_WithPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	if ip := clientIP(req); ip != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %q", ip)
	}
}

func TestClientIP_MalformedFallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-a-host-port"

	if ip := clientIP(req); ip != "not-a-host-port" {
		t.Errorf("expected fallback to raw RemoteAddr, got %q", ip)
	}
}

func TestRateLimitByIP_Allowed(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(1, 0)
	handler := rateLimitByIP(limiter)(newOKHandler())

	req := httptest.NewRequest(http.MethodPost, "/url", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimitByIP_Denied(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(1, 0)
	handler := rateLimitByIP(limiter)(newOKHandler())

	req := httptest.NewRequest(http.MethodPost, "/url", nil)
	req.RemoteAddr = "203.0.113.5:1"

	handler.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestRateLimitByIP_FailsOpenOnLimiterError(t *testing.T) {
	limiter := &fakeLimiter{err: errors.New("redis unavailable")}
	handler := rateLimitByIP(limiter)(newOKHandler())

	req := httptest.NewRequest(http.MethodPost, "/url", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected fail-open to allow the request (200), got %d", rec.Code)
	}
}

func TestRateLimitByIPAndLink_BothAllowed(t *testing.T) {
	rlIP := ratelimit.NewInMemoryLimiter(10, 0)
	rlLink := ratelimit.NewInMemoryLimiter(10, 0)
	handler := rateLimitByIPAndLink(rlIP, rlLink)(newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/url/abc1234", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimitByIPAndLink_DeniedByIP(t *testing.T) {
	rlIP := &fakeLimiter{allow: false}
	rlLink := &fakeLimiter{allow: true}
	handler := rateLimitByIPAndLink(rlIP, rlLink)(newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/url/abc1234", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestRateLimitByIPAndLink_DeniedByLink(t *testing.T) {
	rlIP := &fakeLimiter{allow: true}
	rlLink := &fakeLimiter{allow: false}
	handler := rateLimitByIPAndLink(rlIP, rlLink)(newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/url/abc1234", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestRateLimitByIPAndLink_FailsOpenOnEitherError(t *testing.T) {
	rlIP := &fakeLimiter{err: errors.New("redis unavailable")}
	rlLink := &fakeLimiter{allow: true}
	handler := rateLimitByIPAndLink(rlIP, rlLink)(newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/url/abc1234", nil)
	req.RemoteAddr = "203.0.113.5:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected fail-open to allow the request (200), got %d", rec.Code)
	}
}
