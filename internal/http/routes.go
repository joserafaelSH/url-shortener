package http

import "net/http"

func CreateShortURL(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	jsonResponse := `{"message": "Testing CreateShortURL endpoint"}`
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(jsonResponse))
	
} 

func GetShortURL(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	jsonResponse := `{"message": "Testing GetShortURL endpoint"}`
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(jsonResponse))
}