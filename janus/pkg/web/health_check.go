package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/hellofresh/janus/pkg/proxy/balancer"
	log "github.com/sirupsen/logrus"
)

type RegisterRequest struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

type serviceRegistryResponse struct {
	Message     string   `json:"message"`
	TotalActive int      `json:"total_active"`
	Services    []string `json:"services"`
}

var (
	registryMu      sync.RWMutex
	registeredNodes = make(map[string]bool)
)

func RegisterServiceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		backendURL := normalizeServiceURL(fmt.Sprintf("http://%s:%s", req.IP, req.Port))

		registryMu.Lock()
		_, exists := registeredNodes[backendURL]
		registeredNodes[backendURL] = true
		registryMu.Unlock()

		if !exists {
			log.Infof("[Registry] Node joined: %s", backendURL)
		}

		syncBalancerHealth()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "registered"})
	}
}

func ListServicesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		services := activeServices()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serviceRegistryResponse{
			Message:     "active services",
			TotalActive: len(services),
			Services:    services,
		})
	}
}

func normalizeServiceURL(service string) string {
	u, err := url.Parse(service)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return service
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func activeServices() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	services := make([]string, 0, len(registeredNodes))
	for service := range registeredNodes {
		services = append(services, service)
	}
	sort.Strings(services)

	return services
}

func syncBalancerHealth() {
	services := activeServices()
	active := make(map[string]bool, len(services))
	for _, service := range services {
		active[service] = true
	}

	balancer.HealthMutex.Lock()
	defer balancer.HealthMutex.Unlock()

	for service, wasAlive := range balancer.ActiveNodes {
		if wasAlive && !active[service] {
			log.Warnf("[Health Check] FAILURE: %s was removed from registry. Marked as DOWN.", service)
		}
		balancer.ActiveNodes[service] = false
	}

	for service := range active {
		if !balancer.ActiveNodes[service] {
			log.Infof("[Health Check] RECOVERY: %s is active in registry.", service)
		}
		balancer.ActiveNodes[service] = true
	}
}

func StartActiveHealthCheck() {
	client := &http.Client{Timeout: 2 * time.Second}

	go func() {
		log.Info("[Health Check] Starting Janus internal backend registry monitoring...")

		for {
			for _, service := range activeServices() {
				resp, err := client.Get(service)
				if resp != nil {
					resp.Body.Close()
				}

				if err != nil || resp.StatusCode != http.StatusOK {
					registryMu.Lock()
					if registeredNodes[service] {
						delete(registeredNodes, service)
						log.Warnf("[Registry] Auto removed unreachable node: %s", service)
					}
					registryMu.Unlock()
				}
			}

			syncBalancerHealth()
			time.Sleep(5 * time.Second)
		}
	}()
}

func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := make(map[string]string)

		balancer.HealthMutex.RLock()
		for service, isAlive := range balancer.ActiveNodes {
			if isAlive {
				results[service] = "UP"
			} else {
				results[service] = "DOWN"
			}
		}
		balancer.HealthMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		prettyJSON, _ := json.MarshalIndent(results, "", "  ")
		w.Write(prettyJSON)
	}
}
