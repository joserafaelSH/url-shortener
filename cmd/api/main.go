package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joserafaelSH/url_shortener/internal/cache"
	"github.com/joserafaelSH/url_shortener/internal/config"
	"github.com/joserafaelSH/url_shortener/internal/database/postgres"
	"github.com/joserafaelSH/url_shortener/internal/http"
	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
	"github.com/joserafaelSH/url_shortener/internal/modules/url"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	client, err := cache.ConnectRedis(cfg.RedisAddr, cfg.ConnectTimeout, cfg.ConnectRetryAttempts, cfg.ConnectRetryBackoff)
	if err != nil {
		slog.Error("failed to connect to Redis", "attempts", cfg.ConnectRetryAttempts, "error", err)
		os.Exit(1)
	}

	databaseConnections := map[int64]string{
		1: cfg.PostgresDSNRegion1,
		2: cfg.PostgresDSNRegion2,
	}

	if _, ok := databaseConnections[cfg.RegionID]; !ok {
		slog.Error("REGION_ID has no matching entry in the configured Postgres DSNs", "region_id", cfg.RegionID)
		os.Exit(1)
	}

	router, err := postgres.NewRegionalRouter(databaseConnections, cfg.ConnectTimeout, cfg.ConnectRetryAttempts, cfg.ConnectRetryBackoff)
	if err != nil {
		slog.Error("failed to connect to Postgres databases", "error", err)
		os.Exit(1)
	}

	urlRepository := url.NewPostgresURLRepository(router, cfg.RepositoryQueryTimeout)

	generator, err := hash.NewGenerator(cfg.NodeID, cfg.RegionID)
	if err != nil {
		slog.Error("failed to create ID generator", "error", err)
		os.Exit(1)
	}

	createURLService := url.NewCreateURLService(urlRepository, generator, cfg.DefaultLinkLifetime)
	getURLService := url.NewGetURLService(urlRepository)

	rlPost := ratelimit.NewRedisLimiter(client, "post", cfg.RateLimitPostMaxTokens, cfg.RateLimitPostRefillRate)
	rlIP := ratelimit.NewRedisLimiter(client, "get:ip", cfg.RateLimitGetIPMaxTokens, cfg.RateLimitGetIPRefillRate)
	rlLink := ratelimit.NewRedisLimiter(client, "get:link", cfg.RateLimitGetLinkMaxTokens, cfg.RateLimitGetLinkRefillRate)

	healthCheck := func(ctx context.Context) error {
		if err := client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis: %w", err)
		}
		return router.Ping(ctx)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("server started", "port", cfg.Port)
	if err := http.StartHttpServer(ctx, cfg.Port, cfg.CORSAllowedOrigins, cfg.MaxRequestBodyBytes, healthCheck, rlPost, rlIP, rlLink, createURLService, getURLService); err != nil {
		slog.Error("server error", "error", err)
	}

	slog.Info("shutting down, closing connections")
	router.Close()
	client.Close()
}
