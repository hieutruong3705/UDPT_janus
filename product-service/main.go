package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	stressMu      sync.Mutex
	stressPayload [][]byte
)

func products(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":  "product-service",
		"products": []string{"Cup", "Bottle", "Glass"},
	})
}

func crash(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "product-service is crashing for demo"})

	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(1)
	}()
}

func stress(w http.ResponseWriter, r *http.Request) {
	block := make([]byte, 8*1024*1024)
	for i := range block {
		block[i] = byte(i)
	}

	stressMu.Lock()
	stressPayload = append(stressPayload, block)
	allocatedMB := len(stressPayload) * 8
	stressMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "product-service accepted stress memory block",
		"allocated_mb": allocatedMB,
	})
}

func main() {
	registryURL := os.Getenv("REGISTRY_URLS")
	if registryURL == "" {
		registryURL = os.Getenv("REGISTRY_URL")
	}
	if registryURL == "" {
		registryURL = "http://127.0.0.1:8880/api/register,http://127.0.0.1:8890/api/register"
	}

	myHost := os.Getenv("SERVICE_HOST")
	if myHost == "" {
		myHost = "127.0.0.1"
	}

	myPort := os.Getenv("SERVICE_PORT")
	if myPort == "" {
		myPort = "9002"
	}

	go autoRegister(myHost, myPort, registryURL)

	http.HandleFunc("/crash", crash)
	http.HandleFunc("/stress", stress)
	http.HandleFunc("/", products)

	println("Product Service is running on port :" + myPort)
	http.ListenAndServe(":"+myPort, nil)
}
