package main

import (
	"encoding/json"
	"net/http"
)

func products(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "product-service",
		"users": []string{
			"Cup",
			"Bottle",
			"Glass",
		},
	})
}

func main() {
	http.HandleFunc("/", products)

	println("Product Service :9002")

	http.ListenAndServe(":9002", nil)
}
