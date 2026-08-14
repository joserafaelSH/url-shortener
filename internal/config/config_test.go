package config

import (
	"testing"
	"time"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NODE_ID", "3")
	t.Setenv("REGION_ID", "1")
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}

	if cfg.Port != "3000" {
		t.Errorf("expected default Port=3000, got %q", cfg.Port)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("expected default RedisAddr=localhost:6379, got %q", cfg.RedisAddr)
	}
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("expected default ConnectTimeout=5s, got %v", cfg.ConnectTimeout)
	}
	if cfg.ConnectRetryAttempts != 5 {
		t.Errorf("expected default ConnectRetryAttempts=5, got %d", cfg.ConnectRetryAttempts)
	}
	if cfg.ConnectRetryBackoff != 5*time.Second {
		t.Errorf("expected default ConnectRetryBackoff=5s, got %v", cfg.ConnectRetryBackoff)
	}
	if cfg.RepositoryQueryTimeout != 5*time.Second {
		t.Errorf("expected default RepositoryQueryTimeout=5s, got %v", cfg.RepositoryQueryTimeout)
	}
	if cfg.RateLimitPostMaxTokens != 10 || cfg.RateLimitPostRefillRate != 1.0 {
		t.Errorf("unexpected default POST rate limit: %v/%v", cfg.RateLimitPostMaxTokens, cfg.RateLimitPostRefillRate)
	}
	if cfg.RateLimitGetIPMaxTokens != 100 || cfg.RateLimitGetIPRefillRate != 5.0 {
		t.Errorf("unexpected default GET-IP rate limit: %v/%v", cfg.RateLimitGetIPMaxTokens, cfg.RateLimitGetIPRefillRate)
	}
	if cfg.RateLimitGetLinkMaxTokens != 50 || cfg.RateLimitGetLinkRefillRate != 2.0 {
		t.Errorf("unexpected default GET-link rate limit: %v/%v", cfg.RateLimitGetLinkMaxTokens, cfg.RateLimitGetLinkRefillRate)
	}
	if cfg.DefaultLinkLifetime != 30*24*time.Hour {
		t.Errorf("expected default DefaultLinkLifetime=720h, got %v", cfg.DefaultLinkLifetime)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "https://*" || cfg.CORSAllowedOrigins[1] != "http://*" {
		t.Errorf("unexpected default CORSAllowedOrigins: %v", cfg.CORSAllowedOrigins)
	}
	if cfg.MaxRequestBodyBytes != 16384 {
		t.Errorf("expected default MaxRequestBodyBytes=16384, got %d", cfg.MaxRequestBodyBytes)
	}
	if cfg.NodeID != 3 {
		t.Errorf("expected NodeID=3, got %d", cfg.NodeID)
	}
	if cfg.RegionID != 1 {
		t.Errorf("expected RegionID=1, got %d", cfg.RegionID)
	}
}

func TestLoad_Overrides(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PORT", "8080")
	t.Setenv("REDIS_ADDR", "redis.internal:6380")
	t.Setenv("CONNECT_TIMEOUT", "2s")
	t.Setenv("CONNECT_RETRY_ATTEMPTS", "3")
	t.Setenv("CONNECT_RETRY_BACKOFF", "1s")
	t.Setenv("REPOSITORY_QUERY_TIMEOUT", "10s")
	t.Setenv("RATE_LIMIT_POST_MAX_TOKENS", "20")
	t.Setenv("RATE_LIMIT_POST_REFILL_RATE", "2.5")
	t.Setenv("DEFAULT_LINK_LIFETIME", "24h")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com, https://foo.com")
	t.Setenv("MAX_REQUEST_BODY_BYTES", "2048")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected Port=8080, got %q", cfg.Port)
	}
	if cfg.RedisAddr != "redis.internal:6380" {
		t.Errorf("expected overridden RedisAddr, got %q", cfg.RedisAddr)
	}
	if cfg.ConnectTimeout != 2*time.Second {
		t.Errorf("expected ConnectTimeout=2s, got %v", cfg.ConnectTimeout)
	}
	if cfg.ConnectRetryAttempts != 3 {
		t.Errorf("expected ConnectRetryAttempts=3, got %d", cfg.ConnectRetryAttempts)
	}
	if cfg.ConnectRetryBackoff != time.Second {
		t.Errorf("expected ConnectRetryBackoff=1s, got %v", cfg.ConnectRetryBackoff)
	}
	if cfg.RepositoryQueryTimeout != 10*time.Second {
		t.Errorf("expected RepositoryQueryTimeout=10s, got %v", cfg.RepositoryQueryTimeout)
	}
	if cfg.RateLimitPostMaxTokens != 20 || cfg.RateLimitPostRefillRate != 2.5 {
		t.Errorf("unexpected overridden POST rate limit: %v/%v", cfg.RateLimitPostMaxTokens, cfg.RateLimitPostRefillRate)
	}
	if cfg.DefaultLinkLifetime != 24*time.Hour {
		t.Errorf("expected DefaultLinkLifetime=24h, got %v", cfg.DefaultLinkLifetime)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "https://example.com" || cfg.CORSAllowedOrigins[1] != "https://foo.com" {
		t.Errorf("unexpected overridden CORSAllowedOrigins: %v", cfg.CORSAllowedOrigins)
	}
	if cfg.MaxRequestBodyBytes != 2048 {
		t.Errorf("expected MaxRequestBodyBytes=2048, got %d", cfg.MaxRequestBodyBytes)
	}
}

func TestLoad_InvalidMaxRequestBodyBytes(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAX_REQUEST_BODY_BYTES", "not-an-int")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a malformed MAX_REQUEST_BODY_BYTES")
	}
}

func TestLoad_MissingNodeID(t *testing.T) {
	t.Setenv("REGION_ID", "1")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a missing NODE_ID")
	}
}

func TestLoad_InvalidRegionID(t *testing.T) {
	t.Setenv("NODE_ID", "1")
	t.Setenv("REGION_ID", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a non-numeric REGION_ID")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CONNECT_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a malformed CONNECT_TIMEOUT")
	}
}

func TestLoad_InvalidFloat(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RATE_LIMIT_POST_MAX_TOKENS", "not-a-float")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a malformed RATE_LIMIT_POST_MAX_TOKENS")
	}
}

func TestLoad_InvalidInt(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CONNECT_RETRY_ATTEMPTS", "not-an-int")

	_, err := Load()
	if err == nil {
		t.Error("expected an error for a malformed CONNECT_RETRY_ATTEMPTS")
	}
}
