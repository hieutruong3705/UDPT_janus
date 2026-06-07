package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type RegisterRequest struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

var (
	activeNodes = make(map[string]bool)
	mu          sync.Mutex
)

// 1. API Nhận đăng ký
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	json.NewDecoder(r.Body).Decode(&req)
	backendURL := fmt.Sprintf("http://%s:%s", req.IP, req.Port)

	mu.Lock()
	activeNodes[backendURL] = true
	mu.Unlock()

	fmt.Printf("[+] Nhận tín hiệu: Dịch vụ %s vừa gia nhập mạng lưới!\n", backendURL)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Đăng ký thành công"})
}

// 2. API Xem danh sách
func listServicesHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	var nodes []string
	for node := range activeNodes {
		nodes = append(nodes, node)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Danh bạ các Service đang hoạt động",
		"total_active": len(nodes),
		"services":     nodes,
	})
}

// 3. Health Check & Auto Remove
func healthCheck() {
	client := http.Client{Timeout: 2 * time.Second}

	for {
		time.Sleep(5 * time.Second)

		mu.Lock()
		nodesToCheck := make([]string, 0, len(activeNodes))
		for node := range activeNodes {
			nodesToCheck = append(nodesToCheck, node)
		}
		mu.Unlock()

		for _, node := range nodesToCheck {
			resp, err := client.Get(node)

			if err != nil || resp.StatusCode != http.StatusOK {
				mu.Lock()
				if activeNodes[node] {
					delete(activeNodes, node)
					fmt.Printf("❌ [Auto Remove] Node %s không phản hồi. Đã loại bỏ khỏi danh bạ!\n", node)
				}
				mu.Unlock()
			} else {
				if resp != nil {
					resp.Body.Close()
				}
			}
		}
	}
}

func main() {
	// Đã bổ sung đầy đủ các đường dẫn và tính năng tại đây
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/services", listServicesHandler)

	go healthCheck()

	fmt.Println("🚀 Registry Service đang lắng nghe tại cổng 8080")
	http.ListenAndServe(":8080", nil)
}
