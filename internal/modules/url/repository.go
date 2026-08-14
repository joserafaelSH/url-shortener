package url

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joserafaelSH/url_shortener/internal/database/postgres"
	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
)

var ErrURLNotFound = errors.New("short url not found")

type UrlInformationDto struct {
	LongURL   string
	ExpiresAt time.Time
}

type URLRepository interface {
	Create(ctx context.Context, id int64, longURL string, expiresAt time.Time) (UrlInformationDto, error)
	Get(ctx context.Context, id int64) (string, error)
}

// --- implementação in-memory, para testes sem Postgres real ---

type InMemoryURLRepository struct {
	mu   sync.Mutex
	data map[int64]UrlInformationDto
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{data: make(map[int64]UrlInformationDto)}
}

func (repo *InMemoryURLRepository) Create(ctx context.Context, id int64, longURL string, expiresAt time.Time) (UrlInformationDto, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	info := UrlInformationDto{LongURL: longURL, ExpiresAt: expiresAt}
	repo.data[id] = info
	return info, nil
}

func (repo *InMemoryURLRepository) Get(ctx context.Context, id int64) (string, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	info, exists := repo.data[id]
	if !exists {
		return "", fmt.Errorf("Get: id %d: %w", id, ErrURLNotFound)
	}
	if !info.ExpiresAt.IsZero() && info.ExpiresAt.Before(time.Now()) {
		return "", fmt.Errorf("Get: id %d: %w", id, ErrURLNotFound)
	}
	return info.LongURL, nil
}

// --- implementação Postgres real ---

type PostgresURLRepository struct {
	router       *postgres.RegionalRouter
	queryTimeout time.Duration
}

func NewPostgresURLRepository(router *postgres.RegionalRouter, queryTimeout time.Duration) *PostgresURLRepository {
	return &PostgresURLRepository{router: router, queryTimeout: queryTimeout}
}

func (repo *PostgresURLRepository) Create(ctx context.Context, id int64, longURL string, expiresAt time.Time) (UrlInformationDto, error) {
	regionID := (id >> hash.RegionShift) & hash.MaxRegionID
	pool, err := repo.router.GetPool(regionID)
	if err != nil {
		return UrlInformationDto{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, repo.queryTimeout)
	defer cancel()

	_, err = pool.Exec(ctx,
		"INSERT INTO short_urls (id, long_url, expires_at) VALUES ($1, $2, $3)",
		id, longURL, expiresAt,
	)
	if err != nil {
		return UrlInformationDto{}, fmt.Errorf("failed to execute query (%d): %w", regionID, err)
	}

	return UrlInformationDto{LongURL: longURL, ExpiresAt: expiresAt}, nil
}

func (repo *PostgresURLRepository) Get(ctx context.Context, id int64) (string, error) {
	regionID := (id >> hash.RegionShift) & hash.MaxRegionID
	pool, err := repo.router.GetPool(regionID)
	if err != nil {
		return "", fmt.Errorf("failed to get connection pool (%d): %w", regionID, err)
	}

	ctx, cancel := context.WithTimeout(ctx, repo.queryTimeout)
	defer cancel()

	var longURL string
	err = pool.QueryRow(ctx,
		"SELECT long_url FROM short_urls WHERE id = $1 AND (expires_at IS NULL OR expires_at > now())",
		id,
	).Scan(&longURL)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("Get: id %d: %w", id, ErrURLNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("failed to execute query (%d): %w", regionID, err)
	}

	return longURL, nil
}