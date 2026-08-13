package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
)

func StartHttpServer(port string, rlPost *ratelimit.RedisLimiter, rlIP *ratelimit.RedisLimiter, rlLink *ratelimit.RedisLimiter) {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins:   []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
 	 }))
	r.Use(middleware.Logger)
	r.With(rateLimitByIP(rlPost)).Post("/url", func(w http.ResponseWriter, r *http.Request) {
		CreateShortURL(w, r)
	})

	r.With(rateLimitByIPAndLink(rlIP, rlLink)).Get("/url/{id}", func(w http.ResponseWriter, r *http.Request) {
		GetShortURL(w, r)
	})
	
	http.ListenAndServe(":" + port, r)
}