package main

import (
	"encoding/json"
	"net/http"
)

func products(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"Laptop", "Bicycle", "Keyboard"})
}

func main() {
	http.HandleFunc("/", products)

	http.ListenAndServe(":9002", nil)
}
