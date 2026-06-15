package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
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

type clusterStatusResponse struct {
	NodeID     string   `json:"node_id"`
	LeaderID   string   `json:"leader_id"`
	IsLeader   bool     `json:"is_leader"`
	AliveNodes []string `json:"alive_nodes"`
	Peers      []string `json:"peers"`
}

var (
	registryMu      sync.RWMutex
	registeredNodes = make(map[string]bool)

	clusterMu  sync.RWMutex
	nodeID     = resolveNodeID()
	leaderID   = nodeID
	aliveNodes = []string{nodeID}
	peerURLs   = parsePeerURLs(os.Getenv("JANUS_PEERS"))

	stressMu      sync.Mutex
	stressPayload [][]byte
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

func ClusterStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := currentClusterStatus()
		summary := map[string]interface{}{
			"node_id":   status.NodeID,
			"leader_id": status.LeaderID,
			"is_leader": status.IsLeader,
		}

		w.Header().Set("Content-Type", "application/json")
		prettyJSON, _ := json.MarshalIndent(summary, "", "  ")
		w.Write(prettyJSON)
	}
}

func ClusterCrashHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Janus node is crashing for demo",
			"node_id": nodeID,
		})

		go func() {
			time.Sleep(200 * time.Millisecond)
			log.Warnf("[Cluster] Demo crash requested for node %s", nodeID)
			os.Exit(1)
		}()
	}
}

func ClusterStressHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		block := make([]byte, 16*1024*1024)
		for i := range block {
			block[i] = byte(i)
		}

		stressMu.Lock()
		stressPayload = append(stressPayload, block)
		allocatedMB := len(stressPayload) * 16
		stressMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "Janus node accepted stress memory block",
			"node_id":      nodeID,
			"allocated_mb": allocatedMB,
		})
	}
}

func resolveNodeID() string {
	if id := os.Getenv("JANUS_NODE_ID"); id != "" {
		return id
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "janus-1"
	}
	return hostname
}

func parsePeerURLs(raw string) map[string]string {
	peers := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			log.Warnf("[Cluster] Ignoring invalid JANUS_PEERS item: %s", item)
			continue
		}

		peerID := strings.TrimSpace(parts[0])
		peerURL := strings.TrimRight(strings.TrimSpace(parts[1]), "/")
		if peerID == "" || peerURL == "" {
			log.Warnf("[Cluster] Ignoring invalid JANUS_PEERS item: %s", item)
			continue
		}

		peers[peerID] = peerURL
	}
	return peers
}

func currentClusterStatus() clusterStatusResponse {
	clusterMu.RLock()
	defer clusterMu.RUnlock()

	peers := make([]string, 0, len(peerURLs))
	for peerID := range peerURLs {
		peers = append(peers, peerID)
	}
	sort.Strings(peers)

	nodes := append([]string(nil), aliveNodes...)

	return clusterStatusResponse{
		NodeID:     nodeID,
		LeaderID:   leaderID,
		IsLeader:   leaderID == nodeID,
		AliveNodes: nodes,
		Peers:      peers,
	}
}

func electLeader(client *http.Client) {
	nodes := []string{nodeID}

	for peerID, peerURL := range peerURLs {
		resp, err := client.Get(peerURL + "/cluster/status")
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			nodes = append(nodes, peerID)
		}
	}

	sort.Strings(nodes)
	nextLeader := nodes[0]

	clusterMu.Lock()
	leaderChanged := leaderID != nextLeader
	leaderID = nextLeader
	aliveNodes = nodes
	clusterMu.Unlock()

	if leaderChanged {
		log.Infof("[Cluster] Leader elected: %s", nextLeader)
	}
}

func currentLeaderID() string {
	clusterMu.RLock()
	defer clusterMu.RUnlock()
	return leaderID
}

func isCurrentNodeLeader() bool {
	return currentLeaderID() == nodeID
}

func syncFromLeader(client *http.Client) {
	leader := currentLeaderID()
	if leader == nodeID {
		return
	}

	leaderURL := peerURLs[leader]
	if leaderURL == "" {
		return
	}

	resp, err := client.Get(leaderURL + "/api/services")
	if err != nil {
		log.WithError(err).Warnf("[Cluster] Could not sync registry from leader %s", leader)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warnf("[Cluster] Leader %s returned status %d while syncing registry", leader, resp.StatusCode)
		return
	}

	var payload serviceRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		log.WithError(err).Warnf("[Cluster] Could not decode registry from leader %s", leader)
		return
	}

	setActiveServices(payload.Services)
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

func setActiveServices(services []string) {
	registryMu.Lock()
	registeredNodes = make(map[string]bool, len(services))
	for _, service := range services {
		registeredNodes[normalizeServiceURL(service)] = true
	}
	registryMu.Unlock()

	syncBalancerHealth()
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
		log.WithFields(log.Fields{
			"node_id": nodeID,
			"peers":   peerURLs,
		}).Info("[Cluster] Starting Janus leader election and backend monitoring...")

		for {
			electLeader(client)

			if isCurrentNodeLeader() {
				checkRegisteredBackends(client)
			} else {
				syncFromLeader(client)
			}

			syncBalancerHealth()
			time.Sleep(5 * time.Second)
		}
	}()
}

func checkRegisteredBackends(client *http.Client) {
	for _, service := range activeServices() {
		resp, err := client.Get(service)
		if resp != nil {
			resp.Body.Close()
		}

		if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
			registryMu.Lock()
			if registeredNodes[service] {
				delete(registeredNodes, service)
				log.Warnf("[Registry] Auto removed unreachable node: %s", service)
			}
			registryMu.Unlock()
		}
	}
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
