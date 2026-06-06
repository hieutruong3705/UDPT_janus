package main

import (
	"encoding/json"
	"net/http"
)

func users(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "user-service",
		"users": []string{
			"Hai",
			"An",
			"Binh",
		},
	})
}

func main() {
	http.HandleFunc("/", users)

	println("User Service :9001")

	http.ListenAndServe(":9001", nil)
}
