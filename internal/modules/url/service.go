package url

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/joserafaelSH/url_shortener/internal/modules/hash"
)

var ErrInvalidURL = errors.New("invalid long url")
var ErrInvalidShortID = errors.New("invalid short id format")

type CreatedURLDto struct {
	EncodedShortID string
	ExpiresAt      time.Time
}

type CreateURLService struct {
	repo         URLRepository
	generator    *hash.Generator
	linkLifetime time.Duration
}

func NewCreateURLService(repo URLRepository, generator *hash.Generator, linkLifetime time.Duration) *CreateURLService {
	return &CreateURLService{repo: repo, generator: generator, linkLifetime: linkLifetime}
}

func (s *CreateURLService) Execute(ctx context.Context, longURL string) (CreatedURLDto, error) {
	parsed, err := url.ParseRequestURI(longURL)
	if err != nil {
		return CreatedURLDto{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return CreatedURLDto{}, fmt.Errorf("%w: scheme %q is not allowed", ErrInvalidURL, parsed.Scheme)
	}

	newID, err := s.generator.NextID()
	if err != nil {
		return CreatedURLDto{}, fmt.Errorf("failed to generate ID: %w", err)
	}

	// decisão de negócio (por quanto tempo um link vive) mora aqui, não no
	// repositório -- calculada uma única vez, reutilizada em toda a função
	expiresAt := time.Now().Add(s.linkLifetime)

	urlInformation, err := s.repo.Create(ctx, newID, parsed.String(), expiresAt)
	if err != nil {
		return CreatedURLDto{}, fmt.Errorf("failed to create URL: %w", err)
	}

	encodedShortID := hash.EncodeBase62(newID)
	return CreatedURLDto{
		EncodedShortID: encodedShortID,
		ExpiresAt:      urlInformation.ExpiresAt,
	}, nil
}

type GetURLService struct {
	repo URLRepository
}

func NewGetURLService(repo URLRepository) *GetURLService {
	return &GetURLService{repo: repo}
}

func (s *GetURLService) Execute(ctx context.Context, shortID string) (string, error) {
	id, err := hash.DecodeBase62(shortID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidShortID, err)
	}

	longURL, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrURLNotFound) {
			return "", fmt.Errorf("short URL not found: %w", err)
		}
		return "", fmt.Errorf("failed to get URL: %w", err)
	}
	return longURL, nil
}