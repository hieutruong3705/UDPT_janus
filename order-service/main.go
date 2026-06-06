package main

import (
	"encoding/json"
	"net/http"
)

func orders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "order-service",
		"users": []string{
			"Oder_1",
			"Oder_2",
			"Oder_3",
		},
	})
}

func main() {
	http.HandleFunc("/", orders)

	println("Oder Service :9003")

	http.ListenAndServe(":9003", nil)
}
