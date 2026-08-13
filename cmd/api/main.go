package main

import (
	"log"
	"time"

	"github.com/joserafaelSH/url_shortener/internal/cache"
	"github.com/joserafaelSH/url_shortener/internal/http"
	"github.com/joserafaelSH/url_shortener/internal/modules/ratelimit"
)

func main() {
	client, err := cache.ConnectRedis("localhost:6379")
	if err != nil {
		tries := 1 
		log.Println("Failed to connect to Redis (try ", tries, "):", err)
		tries++
		for tries <= 5 {
			time.Sleep(5 * time.Second)
			client, err = cache.ConnectRedis("localhost:6379")
			if err == nil {
				break
			}
			log.Println("Failed to connect to Redis (try ", tries, "):", err)
			tries++
		}
		if err != nil {
			log.Panic("Failed to connect to Redis after 5 attempts: " + err.Error())
}
	}
	rlPost := ratelimit.NewRedisLimiter(client, "post", 10, 1.0)
	rlIP := ratelimit.NewRedisLimiter(client, "get:ip", 100, 5.0)
	rlLink := ratelimit.NewRedisLimiter(client, "get:link", 50, 2.0)
	log.Println("Server started on port 3000")
	http.StartHttpServer("3000", rlPost, rlIP, rlLink)
}
