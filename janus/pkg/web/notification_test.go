package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetNotificationTestState(t *testing.T) {
	t.Helper()

	oldNotifications := notifications
	oldPeerURLs := peerURLs
	oldNodeID := nodeID

	notifications = newNotificationHub()
	peerURLs = map[string]string{}
	nodeID = "janus-test"

	t.Cleanup(func() {
		notifications = oldNotifications
		peerURLs = oldPeerURLs
		nodeID = oldNodeID
	})
}

func TestNotificationPublishAndHistory(t *testing.T) {
	resetNotificationTestState(t)

	body := bytes.NewBufferString(`{
		"topic": "orders",
		"event_type": "ORDER_CREATED",
		"source": "order-service",
		"payload": {"order_id": 101}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/notifications/publish", body)
	rec := httptest.NewRecorder()

	NotificationPublishHandler()(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	historyReq := httptest.NewRequest(http.MethodGet, "/notifications/history?topic=orders", nil)
	historyRec := httptest.NewRecorder()
	NotificationHistoryHandler()(historyRec, historyReq)

	require.Equal(t, http.StatusOK, historyRec.Code)

	var payload struct {
		Count int            `json:"count"`
		Items []Notification `json:"items"`
	}
	require.NoError(t, json.NewDecoder(historyRec.Body).Decode(&payload))
	assert.Equal(t, 1, payload.Count)
	assert.Equal(t, "ORDER_CREATED", payload.Items[0].EventType)
	assert.Equal(t, "orders", payload.Items[0].Topic)
	assert.Equal(t, "order-service", payload.Items[0].Source)
}

func TestNotificationPublishRequiresTopic(t *testing.T) {
	resetNotificationTestState(t)

	req := httptest.NewRequest(http.MethodPost, "/notifications/publish", bytes.NewBufferString(`{"event_type":"CUSTOM"}`))
	rec := httptest.NewRecorder()

	NotificationPublishHandler()(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
