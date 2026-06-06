package web

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthInfo lưu thông tin trạng thái của 1 service
type HealthInfo struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
}

// targetServices chứa danh sách các microservice cần monitor
// Để đơn giản cho BTL, chúng ta định nghĩa sẵn các node
var targetServices = map[string]string{
	"order-service":   "http://order-service:9003",
	"product-service": "http://product-service:9002",
	"user-service":    "http://user-service:9001",
}

// checkService thực hiện ping đến service và đo độ trễ
func checkService(url string) HealthInfo {
	client := http.Client{
		Timeout: 2 * time.Second, // Timeout ngắn để Gateway không bị treo
	}

	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil || resp.StatusCode >= 500 {
		return HealthInfo{Status: "DOWN"}
	}
	defer resp.Body.Close()

	return HealthInfo{
		Status:  "UP",
		Latency: latency,
	}
}

// HealthCheckHandler là endpoint sẽ được gọi từ Admin API
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := make(map[string]HealthInfo)
		for name, url := range targetServices {
			results[name] = checkService(url)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		prettyJSON, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(prettyJSON)
	}
}
