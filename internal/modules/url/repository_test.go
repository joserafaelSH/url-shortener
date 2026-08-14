package url

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryURLRepository_CreateAndGet(t *testing.T) {
	repo := NewInMemoryURLRepository()

	info, err := repo.Create(context.Background(), 1, "https://example.com", time.Time{})
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if info.LongURL != "https://example.com" {
		t.Errorf("expected https://example.com, got %q", info.LongURL)
	}

	longURL, err := repo.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if longURL != "https://example.com" {
		t.Errorf("expected https://example.com, got %q", longURL)
	}
}

func TestInMemoryURLRepository_Get_NotFound(t *testing.T) {
	repo := NewInMemoryURLRepository()

	_, err := repo.Get(context.Background(), 999)
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound, got: %v", err)
	}
}

func TestInMemoryURLRepository_Get_Expired(t *testing.T) {
	repo := NewInMemoryURLRepository()

	if _, err := repo.Create(context.Background(), 1, "https://example.com", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := repo.Get(context.Background(), 1)
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound for an expired entry, got: %v", err)
	}
}

func TestInMemoryURLRepository_Get_NoExpiry(t *testing.T) {
	repo := NewInMemoryURLRepository()

	if _, err := repo.Create(context.Background(), 1, "https://example.com", time.Time{}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := repo.Get(context.Background(), 1); err != nil {
		t.Errorf("expected a zero ExpiresAt to mean 'never expires', got: %v", err)
	}
}

func TestInMemoryURLRepository_IndependentEntries(t *testing.T) {
	repo := NewInMemoryURLRepository()

	if _, err := repo.Create(context.Background(), 1, "https://one.example.com", time.Time{}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := repo.Create(context.Background(), 2, "https://two.example.com", time.Time{}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	one, err := repo.Get(context.Background(), 1)
	if err != nil || one != "https://one.example.com" {
		t.Errorf("expected https://one.example.com, got %q (err: %v)", one, err)
	}

	two, err := repo.Get(context.Background(), 2)
	if err != nil || two != "https://two.example.com" {
		t.Errorf("expected https://two.example.com, got %q (err: %v)", two, err)
	}
}
