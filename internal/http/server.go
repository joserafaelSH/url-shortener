package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
	"github.com/joserafaelSH/url_shortener/internal/modules/url"
)

func NewRouter(allowedOrigins []string, maxRequestBodyBytes int64, healthCheck func(context.Context) error, rlPost ratelimit.Limiter, rlIP ratelimit.Limiter, rlLink ratelimit.Limiter, createURLService *url.CreateURLService, getURLService *url.GetURLService) http.Handler {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: allowedOrigins,
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Use(middleware.Logger)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := healthCheck(ctx); err != nil {
			http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.With(rateLimitByIP(rlPost), maxBodySize(maxRequestBodyBytes)).Post("/url", func(w http.ResponseWriter, r *http.Request) {
		CreateShortURL(w, r, createURLService)
	})

	r.With(rateLimitByIPAndLink(rlIP, rlLink)).Get("/url/{id}", func(w http.ResponseWriter, r *http.Request) {
		GetShortURL(w, r, getURLService)
	})

	return r
}

func StartHttpServer(ctx context.Context, port string, allowedOrigins []string, maxRequestBodyBytes int64, healthCheck func(context.Context) error, rlPost ratelimit.Limiter, rlIP ratelimit.Limiter, rlLink ratelimit.Limiter, createURLService *url.CreateURLService, getURLService *url.GetURLService) error {
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: NewRouter(allowedOrigins, maxRequestBodyBytes, healthCheck, rlPost, rlIP, rlLink, createURLService, getURLService),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
