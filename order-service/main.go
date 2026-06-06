package main

import (
	"encoding/json"
	"net/http"
)

func orders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"Order001", "Order002"})
}

func main() {
	http.HandleFunc("/", orders)
	http.ListenAndServe(":9003", nil)
}
