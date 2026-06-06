package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hellofresh/janus/pkg/proxy/balancer" // Gọi anh Bảo vệ vào
	log "github.com/sirupsen/logrus"
)

type HealthInfo struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
}

var targetServices = map[string]string{
	"user-service":      "http://user-service:9001/users",
	"product-service-1": "http://product-service-1:9002/products",
	"product-service-2": "http://product-service-2:9002/products",
	"order-service-1":   "http://order-service-1:9003/orders",
	"order-service-2":   "http://order-service-2:9003/orders",
}

func checkService(url string) HealthInfo {
	client := http.Client{Timeout: 1 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil || resp.StatusCode >= 500 {
		return HealthInfo{Status: "DOWN"}
	}
	defer resp.Body.Close()

	return HealthInfo{Status: "UP", Latency: latency}
}

func StartActiveHealthCheck() {
	// Ghi trạng thái ban đầu vào sổ tay của balancer
	for name := range targetServices {
		balancer.HealthMutex.Lock()
		balancer.ActiveNodes[name] = true
		balancer.HealthMutex.Unlock()
	}

	go func() {
		log.Info("[Health Check] Starting background monitoring...")

		for {
			for name, url := range targetServices {
				info := checkService(url)
				isAlive := (info.Status == "UP")

				// Mượn sổ tay của balancer để ghi chép
				balancer.HealthMutex.Lock()
				wasAlive := balancer.ActiveNodes[name]
				balancer.ActiveNodes[name] = isAlive
				balancer.HealthMutex.Unlock()

				if wasAlive && !isAlive {
					log.Warnf("[Health Check] FAILURE: %s is unreachable. Marked as DOWN.", name)
				} else if !wasAlive && isAlive {
					log.Infof("[Health Check] RECOVERY: %s is back online.", name)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := make(map[string]string)

		// Đọc trực tiếp từ sổ tay của balancer
		balancer.HealthMutex.RLock()
		for name, isAlive := range balancer.ActiveNodes {
			if isAlive {
				results[name] = "UP"
			} else {
				results[name] = "DOWN"
			}
		}
		balancer.HealthMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		prettyJSON, _ := json.MarshalIndent(results, "", "  ")
		w.Write(prettyJSON)
	}
}
