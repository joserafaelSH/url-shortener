package http

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"

	"github.com/go-chi/chi/v5"
)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // fallback, caso não tenha porta por algum motivo
	}
	return host
}

func maxBodySize(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitByIP(rl ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			ok, err := rl.Allow(r.Context(), ip)
			if err != nil {
				// fail-open: Redis fora do ar não deveria derrubar a API inteira
				slog.Warn("rate limiter unavailable, failing open", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if !ok {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("rate limit exceeded"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitByIPAndLink(rlIP ratelimit.Limiter, rlLink ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			shortID := chi.URLParam(r, "id")

			okIP, err := rlIP.Allow(r.Context(), ip)
			if err != nil {
				slog.Warn("rate limiter (IP) unavailable, failing open", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if !okIP {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("rate limit exceeded (client)"))
				return
			}

			okLink, err := rlLink.Allow(r.Context(), shortID)
			if err != nil {
				slog.Warn("rate limiter (link) unavailable, failing open", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if !okLink {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("rate limit exceeded (link)"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
