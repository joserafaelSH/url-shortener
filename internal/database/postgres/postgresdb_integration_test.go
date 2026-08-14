//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func testDSNRegion1() string {
	if dsn := os.Getenv("TEST_POSTGRES_DSN_REGION_1"); dsn != "" {
		return dsn
	}
	return "postgres://admin:admin@localhost:5432/shard_region_1?sslmode=disable"
}

func testDSNRegion2() string {
	if dsn := os.Getenv("TEST_POSTGRES_DSN_REGION_2"); dsn != "" {
		return dsn
	}
	return "postgres://admin:admin@localhost:5433/shard_region_2?sslmode=disable"
}

func newTestRouter(t *testing.T) *RegionalRouter {
	t.Helper()
	router, err := NewRegionalRouter(map[int64]string{
		1: testDSNRegion1(),
		2: testDSNRegion2(),
	}, 5*time.Second, 1, 0)
	if err != nil {
		t.Fatalf("failed to connect to test databases: %v", err)
	}

	for _, regionID := range []int64{1, 2} {
		pool, err := router.GetPool(regionID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if _, err := pool.Exec(context.Background(), Schema); err != nil {
			t.Fatalf("failed to apply schema to region %d: %v", regionID, err)
		}
	}

	return router
}

func TestNewRegionalRouter_ConnectsToConfiguredRegions(t *testing.T) {
	router := newTestRouter(t)

	for _, regionID := range []int64{1, 2} {
		pool, err := router.GetPool(regionID)
		if err != nil {
			t.Fatalf("expected a pool for region %d, got error: %v", regionID, err)
		}
		if err := pool.Ping(context.Background()); err != nil {
			t.Errorf("expected region %d's pool to be reachable, got: %v", regionID, err)
		}
	}
}

func TestRegionalRouter_GetPool_UnconfiguredRegion(t *testing.T) {
	router := newTestRouter(t)

	_, err := router.GetPool(99)
	if !errors.Is(err, ErrRegionNotFound) {
		t.Errorf("expected ErrRegionNotFound, got: %v", err)
	}
}
