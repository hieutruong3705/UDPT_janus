package web

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type NotificationPublishRequest struct {
	EventType string          `json:"event_type"`
	Topic     string          `json:"topic"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
}

type Notification struct {
	ID        string          `json:"id"`
	EventType string          `json:"event_type"`
	Topic     string          `json:"topic"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
	NodeID    string          `json:"node_id"`
	CreatedAt time.Time       `json:"created_at"`
}

type notificationSubscriber struct {
	id     string
	topics map[string]bool
	ch     chan Notification
}

type notificationHub struct {
	mu          sync.RWMutex
	subscribers map[string]*notificationSubscriber
	history     []Notification
	seen        map[string]bool
}

var notifications = newNotificationHub()

func newNotificationHub() *notificationHub {
	return &notificationHub{
		subscribers: make(map[string]*notificationSubscriber),
		history:     make([]Notification, 0, 200),
		seen:        make(map[string]bool),
	}
}

func (h *notificationHub) publish(req NotificationPublishRequest) Notification {
	if req.EventType == "" {
		req.EventType = "CUSTOM"
	}
	if req.Source == "" {
		req.Source = nodeID
	}
	if req.Payload == nil {
		req.Payload = json.RawMessage("null")
	}

	notif := Notification{
		ID:        newNotificationID(),
		EventType: req.EventType,
		Topic:     req.Topic,
		Source:    req.Source,
		Payload:   req.Payload,
		NodeID:    nodeID,
		CreatedAt: time.Now(),
	}

	h.publishNotification(notif)
	return notif
}

func (h *notificationHub) publishNotification(notif Notification) {
	if notif.ID == "" {
		notif.ID = newNotificationID()
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}
	if notif.Payload == nil {
		notif.Payload = json.RawMessage("null")
	}

	h.mu.Lock()
	if h.seen[notif.ID] {
		h.mu.Unlock()
		return
	}
	h.seen[notif.ID] = true

	if len(h.history) >= 200 {
		delete(h.seen, h.history[0].ID)
		h.history = h.history[1:]
	}
	h.history = append(h.history, notif)

	for _, sub := range h.subscribers {
		if sub.topics["*"] || sub.topics[notif.Topic] {
			select {
			case sub.ch <- notif:
			default:
			}
		}
	}
	h.mu.Unlock()
}

func (h *notificationHub) subscribe(topics []string) *notificationSubscriber {
	sub := &notificationSubscriber{
		id:     newNotificationID(),
		topics: make(map[string]bool),
		ch:     make(chan Notification, 50),
	}

	for _, topic := range topics {
		sub.topics[topic] = true
	}

	h.mu.Lock()
	h.subscribers[sub.id] = sub
	h.mu.Unlock()

	return sub
}

func (h *notificationHub) unsubscribe(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sub, ok := h.subscribers[id]; ok {
		close(sub.ch)
		delete(h.subscribers, id)
	}
}

func (h *notificationHub) getHistory(topic string, limit int) []Notification {
	h.mu.RLock()
	defer h.mu.RUnlock()

	items := make([]Notification, 0, len(h.history))
	for _, notif := range h.history {
		if topic == "" || notif.Topic == topic {
			items = append(items, notif)
		}
	}

	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}

	return items
}

func (h *notificationHub) stats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	topics := make(map[string]int)
	for _, sub := range h.subscribers {
		for topic := range sub.topics {
			topics[topic]++
		}
	}

	topicNames := make([]string, 0, len(topics))
	for topic := range topics {
		topicNames = append(topicNames, topic)
	}
	sort.Strings(topicNames)

	return map[string]interface{}{
		"node_id":            nodeID,
		"active_subscribers": len(h.subscribers),
		"history_count":      len(h.history),
		"topics":             topicNames,
	}
}

func NotificationInfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"service": "janus-notification-hub",
			"node_id": nodeID,
			"endpoints": []string{
				"POST /notifications/publish",
				"GET /notifications/subscribe",
				"GET /notifications/history",
				"GET /notifications/stats",
			},
		})
	}
}

func NotificationPublishHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req NotificationPublishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Topic) == "" {
			http.Error(w, "missing topic", http.StatusBadRequest)
			return
		}

		notif := notifications.publish(req)
		go replicateNotification(notif)

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":      "notification published",
			"notification": notif,
		})
	}
}

func NotificationReplicateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var notif Notification
		if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(notif.Topic) == "" {
			http.Error(w, "missing topic", http.StatusBadRequest)
			return
		}

		notifications.publishNotification(notif)
		writeJSON(w, http.StatusOK, map[string]string{"message": "notification replicated"})
	}
}

func NotificationSubscribeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topics := parseNotificationTopics(r.URL.Query().Get("topics"))
		timeout := parseNotificationTimeout(r.URL.Query().Get("timeout"))

		sub := notifications.subscribe(topics)
		defer notifications.unsubscribe(sub.id)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, canFlush := w.(http.Flusher)
		writeSSE(w, "connected", map[string]interface{}{
			"node_id": nodeID,
			"sub_id":  sub.id,
			"topics":  topics,
			"message": "connected to Janus notification hub",
		})
		if canFlush {
			flusher.Flush()
		}

		deadline := time.After(time.Duration(timeout) * time.Second)
		for {
			select {
			case notif, ok := <-sub.ch:
				if !ok {
					return
				}
				writeSSE(w, "notification", notif)
				if canFlush {
					flusher.Flush()
				}
			case <-deadline:
				writeSSE(w, "timeout", map[string]string{
					"message": "subscribe timeout reached",
				})
				if canFlush {
					flusher.Flush()
				}
				return
			case <-r.Context().Done():
				return
			}
		}
	}
}

func NotificationHistoryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		topic := r.URL.Query().Get("topic")
		items := notifications.getHistory(topic, limit)
		if items == nil {
			items = []Notification{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"node_id": nodeID,
			"topic":   topic,
			"count":   len(items),
			"items":   items,
		})
	}
}

func NotificationStatsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, notifications.stats())
	}
}

func replicateNotification(notif Notification) {
	if len(peerURLs) == 0 {
		return
	}

	body, err := json.Marshal(notif)
	if err != nil {
		log.WithError(err).Warn("[Notification] Could not encode notification for replication")
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for peerID, peerURL := range peerURLs {
		resp, err := client.Post(peerURL+"/notifications/replicate", "application/json", bytes.NewReader(body))
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil || resp == nil || resp.StatusCode >= http.StatusBadRequest {
			log.WithError(err).Warnf("[Notification] Could not replicate notification to peer %s", peerID)
		}
	}
}

func parseNotificationTopics(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "*" {
		return []string{"*"}
	}

	parts := strings.Split(raw, ",")
	topics := make([]string, 0, len(parts))
	for _, part := range parts {
		topic := strings.TrimSpace(part)
		if topic != "" {
			topics = append(topics, topic)
		}
	}
	if len(topics) == 0 {
		return []string{"*"}
	}
	return topics
}

func parseNotificationTimeout(raw string) int {
	if raw == "" {
		return 60
	}

	timeout, err := strconv.Atoi(raw)
	if err != nil || timeout <= 0 {
		return 60
	}
	if timeout > 300 {
		return 300
	}
	return timeout
}

func writeSSE(w http.ResponseWriter, event string, data interface{}) {
	payload, _ := json.Marshal(data)
	w.Write([]byte("event: " + event + "\n"))
	w.Write([]byte("data: " + string(payload) + "\n\n"))
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func newNotificationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
