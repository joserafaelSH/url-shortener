package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string

	RedisAddr            string
	ConnectTimeout       time.Duration
	ConnectRetryAttempts int
	ConnectRetryBackoff  time.Duration

	PostgresDSNRegion1     string
	PostgresDSNRegion2     string
	RepositoryQueryTimeout time.Duration

	NodeID   int64
	RegionID int64

	RateLimitPostMaxTokens  float64
	RateLimitPostRefillRate float64

	RateLimitGetIPMaxTokens  float64
	RateLimitGetIPRefillRate float64

	RateLimitGetLinkMaxTokens  float64
	RateLimitGetLinkRefillRate float64

	DefaultLinkLifetime time.Duration

	CORSAllowedOrigins []string

	MaxRequestBodyBytes int64
}

func Load() (*Config, error) {
	cfg := &Config{
		Port: getEnvString("PORT", "3000"),

		RedisAddr: getEnvString("REDIS_ADDR", "localhost:6379"),

		PostgresDSNRegion1: getEnvString("POSTGRES_DSN_REGION_1", "postgres://admin:admin@localhost:5432/shard_region_1?sslmode=disable"),
		PostgresDSNRegion2: getEnvString("POSTGRES_DSN_REGION_2", "postgres://admin:admin@localhost:5433/shard_region_2?sslmode=disable"),

		CORSAllowedOrigins: getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"https://*", "http://*"}),
	}

	var err error
	if cfg.ConnectTimeout, err = getEnvDuration("CONNECT_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ConnectRetryAttempts, err = getEnvInt("CONNECT_RETRY_ATTEMPTS", 5); err != nil {
		return nil, err
	}
	if cfg.ConnectRetryBackoff, err = getEnvDuration("CONNECT_RETRY_BACKOFF", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.RepositoryQueryTimeout, err = getEnvDuration("REPOSITORY_QUERY_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.NodeID, err = requireEnvInt64("NODE_ID"); err != nil {
		return nil, err
	}
	if cfg.RegionID, err = requireEnvInt64("REGION_ID"); err != nil {
		return nil, err
	}
	if cfg.RateLimitPostMaxTokens, err = getEnvFloat("RATE_LIMIT_POST_MAX_TOKENS", 10); err != nil {
		return nil, err
	}
	if cfg.RateLimitPostRefillRate, err = getEnvFloat("RATE_LIMIT_POST_REFILL_RATE", 1.0); err != nil {
		return nil, err
	}
	if cfg.RateLimitGetIPMaxTokens, err = getEnvFloat("RATE_LIMIT_GET_IP_MAX_TOKENS", 100); err != nil {
		return nil, err
	}
	if cfg.RateLimitGetIPRefillRate, err = getEnvFloat("RATE_LIMIT_GET_IP_REFILL_RATE", 5.0); err != nil {
		return nil, err
	}
	if cfg.RateLimitGetLinkMaxTokens, err = getEnvFloat("RATE_LIMIT_GET_LINK_MAX_TOKENS", 50); err != nil {
		return nil, err
	}
	if cfg.RateLimitGetLinkRefillRate, err = getEnvFloat("RATE_LIMIT_GET_LINK_REFILL_RATE", 2.0); err != nil {
		return nil, err
	}
	if cfg.DefaultLinkLifetime, err = getEnvDuration("DEFAULT_LINK_LIFETIME", 30*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.MaxRequestBodyBytes, err = getEnvInt64("MAX_REQUEST_BODY_BYTES", 16384); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnvString(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func getEnvStringSlice(name string, def []string) []string {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnvDuration(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return d, nil
}

func getEnvInt(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return i, nil
}

func getEnvInt64(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return i, nil
}

func getEnvFloat(name string, def float64) (float64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return f, nil
}

func requireEnvInt64(name string) (int64, error) {
	v, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid or missing %s: %w", name, err)
	}
	return v, nil
}
