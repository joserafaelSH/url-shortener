package url

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
)

const testLinkLifetime = time.Hour

func newTestGenerator(t *testing.T) *hash.Generator {
	t.Helper()
	gen, err := hash.NewGenerator(1, 1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	return gen
}

func TestCreateURLService_Execute_ValidURL(t *testing.T) {
	repo := NewInMemoryURLRepository()
	svc := NewCreateURLService(repo, newTestGenerator(t), testLinkLifetime)

	before := time.Now()
	result, err := svc.Execute(context.Background(), "https://example.com/some/path")
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if len(result.EncodedShortID) != 7 {
		t.Errorf("expected a 7-character short ID, got %q", result.EncodedShortID)
	}
	if !result.ExpiresAt.After(before) {
		t.Errorf("expected ExpiresAt to be in the future, got %v", result.ExpiresAt)
	}
}

func TestCreateURLService_Execute_InvalidURL(t *testing.T) {
	repo := NewInMemoryURLRepository()
	svc := NewCreateURLService(repo, newTestGenerator(t), testLinkLifetime)

	_, err := svc.Execute(context.Background(), "not-a-valid-url")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("expected ErrInvalidURL, got: %v", err)
	}
}

func TestCreateURLService_Execute_DisallowedScheme(t *testing.T) {
	cases := []string{"javascript:alert(1)", "ftp://example.com/file"}

	for _, longURL := range cases {
		t.Run(longURL, func(t *testing.T) {
			repo := NewInMemoryURLRepository()
			svc := NewCreateURLService(repo, newTestGenerator(t), testLinkLifetime)

			_, err := svc.Execute(context.Background(), longURL)
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("expected ErrInvalidURL, got: %v", err)
			}
		})
	}
}

func TestCreateURLService_Execute_EmptyURL(t *testing.T) {
	repo := NewInMemoryURLRepository()
	svc := NewCreateURLService(repo, newTestGenerator(t), testLinkLifetime)

	_, err := svc.Execute(context.Background(), "")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("expected ErrInvalidURL, got: %v", err)
	}
}

func TestGetURLService_Execute_RoundTrip(t *testing.T) {
	repo := NewInMemoryURLRepository()
	createSvc := NewCreateURLService(repo, newTestGenerator(t), testLinkLifetime)
	getSvc := NewGetURLService(repo)

	created, err := createSvc.Execute(context.Background(), "https://example.com/foo")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	longURL, err := getSvc.Execute(context.Background(), created.EncodedShortID)
	if err != nil {
		t.Fatalf("did not expect an error, got: %v", err)
	}
	if longURL != "https://example.com/foo" {
		t.Errorf("expected https://example.com/foo, got %q", longURL)
	}
}

func TestGetURLService_Execute_InvalidShortID(t *testing.T) {
	repo := NewInMemoryURLRepository()
	getSvc := NewGetURLService(repo)

	_, err := getSvc.Execute(context.Background(), "$$$$$$$")
	if !errors.Is(err, ErrInvalidShortID) {
		t.Errorf("expected ErrInvalidShortID, got: %v", err)
	}
}

func TestGetURLService_Execute_NotFound(t *testing.T) {
	repo := NewInMemoryURLRepository()
	getSvc := NewGetURLService(repo)

	_, err := getSvc.Execute(context.Background(), "0000000")
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound, got: %v", err)
	}
}

func TestGetURLService_Execute_ExpiredLink(t *testing.T) {
	repo := NewInMemoryURLRepository()
	gen := newTestGenerator(t)
	getSvc := NewGetURLService(repo)

	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := repo.Create(context.Background(), id, "https://example.com/expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	shortID := hash.EncodeBase62(id)
	_, err = getSvc.Execute(context.Background(), shortID)
	if !errors.Is(err, ErrURLNotFound) {
		t.Errorf("expected ErrURLNotFound for an expired link, got: %v", err)
	}
}
