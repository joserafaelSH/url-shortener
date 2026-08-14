package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RegionalRouter struct {
	pools map[int64]*pgxpool.Pool
}

func connectPostgres(connString string, timeout time.Duration) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return pool, nil
}

func NewRegionalRouter(connStrings map[int64]string, timeout time.Duration, retryAttempts int, retryBackoff time.Duration) (*RegionalRouter, error) {
	pools := make(map[int64]*pgxpool.Pool)

	for regionID, connString := range connStrings {
		var pool *pgxpool.Pool
		var err error

		for attempt := 1; attempt <= retryAttempts; attempt++ {
			pool, err = connectPostgres(connString, timeout)
			if err == nil {
				break
			}
			slog.Warn("failed to connect to database, retrying", "region", regionID, "attempt", attempt, "error", err)
			if attempt < retryAttempts {
				time.Sleep(retryBackoff)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database %d after %d attempts: %w", regionID, retryAttempts, err)
		}
		pools[regionID] = pool
	}

	return &RegionalRouter{pools: pools}, nil
}

var ErrRegionNotFound = errors.New("region not found in router")

func (r *RegionalRouter) GetPool(regionID int64) (*pgxpool.Pool, error) {
	if pool, exists := r.pools[regionID]; exists {
		return pool, nil
	}
	return nil, fmt.Errorf("GetPool: region %d: %w", regionID, ErrRegionNotFound)
}

func (r *RegionalRouter) Close() {
	for _, pool := range r.pools {
		pool.Close()
	}
}

func (r *RegionalRouter) Ping(ctx context.Context) error {
	for regionID, pool := range r.pools {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("region %d: %w", regionID, err)
		}
	}
	return nil
}
