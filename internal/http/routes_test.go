package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
	"github.com/joserafaelSH/url_shortener/internal/modules/url"
)

var testAllowedOrigins = []string{"https://*", "http://*"}

const testMaxRequestBodyBytes = 1 << 20 // 1MB, generous for tests

func noopHealthCheck(context.Context) error { return nil }

func newTestServices(t *testing.T) (*url.CreateURLService, *url.GetURLService) {
	t.Helper()
	gen, err := hash.NewGenerator(1, 1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	repo := url.NewInMemoryURLRepository()
	return url.NewCreateURLService(repo, gen, time.Hour), url.NewGetURLService(repo)
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	return NewRouter(testAllowedOrigins, testMaxRequestBodyBytes, noopHealthCheck, allow, allow, allow, createSvc, getSvc)
}

func TestCreateShortURL_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/url", bytes.NewBufferString("{not-json"))
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateShortURL_InvalidLongURL(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"long_url": "not-a-url"})
	req := httptest.NewRequest(http.MethodPost, "/url", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateShortURL_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"long_url": "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/url", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp CreateShortUrlResponseDto
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.ShortURL) != 7 {
		t.Errorf("expected a 7-character short URL, got %q", resp.ShortURL)
	}
	if resp.ExpiresAt == "" {
		t.Error("expected a non-empty expires_at")
	}
}

func TestGetShortURL_RoundTrip(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	router := NewRouter(testAllowedOrigins, testMaxRequestBodyBytes, noopHealthCheck, allow, allow, allow, createSvc, getSvc)

	body, _ := json.Marshal(map[string]string{"long_url": "https://example.com/target"})
	createReq := httptest.NewRequest(http.MethodPost, "/url", bytes.NewBuffer(body))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created CreateShortUrlResponseDto
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/url/"+created.ShortURL, nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", getRec.Code)
	}
	if loc := getRec.Header().Get("Location"); loc != "https://example.com/target" {
		t.Errorf("expected Location=https://example.com/target, got %q", loc)
	}
}

func TestCreateShortURL_BodyTooLarge(t *testing.T) {
	createSvc, getSvc := newTestServices(t)
	allow := ratelimit.NewInMemoryLimiter(1000, 1000)
	router := NewRouter(testAllowedOrigins, 16, noopHealthCheck, allow, allow, allow, createSvc, getSvc)

	body, _ := json.Marshal(map[string]string{"long_url": "https://example.com/a-url-longer-than-sixteen-bytes"})
	req := httptest.NewRequest(http.MethodPost, "/url", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

func TestGetShortURL_InvalidShortID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/url/$$$$$$$", nil)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetShortURL_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/url/0000000", nil)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
