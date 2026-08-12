package main

import (
	"log"

	"github.com/joserafaelSH/url_shortener/internal/http"
)

func main() {
	log.Println("Server started on port 3000")
	http.StartHttpServer("3000")
}
