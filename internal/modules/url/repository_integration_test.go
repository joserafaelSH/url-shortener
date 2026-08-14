//go:build integration

package url

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/joserafaelSH/url_shortener/internal/database/postgres"
	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
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

func newTestPostgresRepo(t *testing.T) *PostgresURLRepository {
	t.Helper()
	router, err := postgres.NewRegionalRouter(map[int64]string{
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
		if _, err := pool.Exec(context.Background(), postgres.Schema); err != nil {
			t.Fatalf("failed to apply schema to region %d: %v", regionID, err)
		}
	}

	return NewPostgresURLRepository(router, 5*time.Second)
}

func TestPostgresURLRepository_CreateAndGet(t *testing.T) {
	repo := newTestPostgresRepo(t)
	gen, err := hash.NewGenerator(5, 1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := repo.Create(context.Background(), id, "https://example.com/pg", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}

	longURL, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if longURL != "https://example.com/pg" {
		t.Errorf("expected https://example.com/pg, got %q", longURL)
	}
}

func TestPostgresURLRepository_Get_NotFound(t *testing.T) {
	repo := newTestPostgresRepo(t)
	gen, err := hash.NewGenerator(6, 1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err = repo.Get(context.Background(), id)
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound, got: %v", err)
	}
}

func TestPostgresURLRepository_Get_Expired(t *testing.T) {
	repo := newTestPostgresRepo(t)
	gen, err := hash.NewGenerator(7, 1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := repo.Create(context.Background(), id, "https://example.com/expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}

	_, err = repo.Get(context.Background(), id)
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound for an expired link, got: %v", err)
	}
}
