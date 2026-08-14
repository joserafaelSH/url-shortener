package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joserafaelSH/url_shortener/internal/modules/url"
)

type CreateShortUrlRequestDto struct {
	LongURL string `json:"long_url" required:"true"`
}

type CreateShortUrlResponseDto struct {
	ShortURL  string `json:"short_url"`
	ExpiresAt string `json:"expires_at"`
}

func CreateShortURL(w http.ResponseWriter, r *http.Request, createURLService *url.CreateURLService) {
	var requestDto CreateShortUrlRequestDto
	if err := json.NewDecoder(r.Body).Decode(&requestDto); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := createURLService.Execute(r.Context(), requestDto.LongURL)
	if err != nil {
		if errors.Is(err, url.ErrInvalidURL) {
			http.Error(w, "Invalid long_url", http.StatusBadRequest)
			return
		}
		slog.Error("failed to create short url", "error", err)
		http.Error(w, "Failed to create short URL", http.StatusInternalServerError)
		return
	}

	response := CreateShortUrlResponseDto{
		ShortURL:  result.EncodedShortID,
		ExpiresAt: result.ExpiresAt.Format(time.RFC3339),
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func GetShortURL(w http.ResponseWriter, r *http.Request, getURLService *url.GetURLService) {
	shortID := chi.URLParam(r, "id")
	if shortID == "" {
		http.Error(w, "Missing short ID", http.StatusBadRequest)
		return
	}

	longURL, err := getURLService.Execute(r.Context(), shortID)
	if err != nil {
		switch {
		case errors.Is(err, url.ErrInvalidShortID):
			http.Error(w, "Invalid short ID", http.StatusBadRequest)
		case errors.Is(err, url.ErrURLNotFound):
			http.Error(w, "Short URL not found", http.StatusNotFound)
		default:
			slog.Error("failed to get short url", "error", err)
			http.Error(w, "Failed to retrieve long URL", http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}