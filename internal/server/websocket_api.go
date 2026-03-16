package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type safeWSConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *safeWSConn) WriteJSON(value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(value)
}

func (c *safeWSConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteControl(messageType, data, deadline)
}

func (a *apiServer) handleTaskWebSocket(w http.ResponseWriter, req *http.Request) {
	taskUUID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/ws/task/"), "/")
	if taskUUID == "" {
		http.NotFound(w, req)
		return
	}

	conn, err := websocketUpgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ws := &safeWSConn{conn: conn}

	if task, err := a.store.GetRegistrationTaskByUUID(req.Context(), taskUUID); err == nil && task != nil {
		_ = ws.WriteJSON(map[string]any{
			"type":      "status",
			"task_uuid": taskUUID,
			"status":    task.Status,
		})
		if strings.TrimSpace(task.Logs) != "" {
			for _, logLine := range strings.Split(task.Logs, "\n") {
				_ = ws.WriteJSON(map[string]any{
					"type":      "log",
					"task_uuid": taskUUID,
					"message":   logLine,
				})
			}
		}
	}

	subscription := a.tasks.SubscribeTask(taskUUID)
	defer a.tasks.UnsubscribeTask(taskUUID, subscription)

	_ = conn.SetReadDeadline(time.Now().Add(35 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(35 * time.Second))
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(35 * time.Second))

			var payload map[string]any
			if json.Unmarshal(message, &payload) != nil {
				continue
			}
			switch payload["type"] {
			case "cancel":
				a.tasks.CancelTask(taskUUID)
				_ = ws.WriteJSON(map[string]any{
					"type":      "status",
					"task_uuid": taskUUID,
					"status":    "cancelling",
					"message":   "取消请求已提交",
				})
			}
		}
	}()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case event := <-subscription:
			_ = ws.WriteJSON(event)
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}

func (a *apiServer) handleBatchWebSocket(w http.ResponseWriter, req *http.Request) {
	batchID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/ws/batch/"), "/")
	if batchID == "" {
		http.NotFound(w, req)
		return
	}

	conn, err := websocketUpgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ws := &safeWSConn{conn: conn}

	if batch := a.tasks.GetBatch(batchID); batch != nil {
		_ = ws.WriteJSON(map[string]any{
			"type":          "status",
			"batch_id":      batchID,
			"status":        batch.Status,
			"total":         batch.Total,
			"completed":     batch.Completed,
			"success":       batch.Success,
			"failed":        batch.Failed,
			"current_index": batch.CurrentIndex,
			"cancelled":     batch.Cancelled,
			"finished":      batch.Finished,
		})
		for _, logLine := range batch.Logs {
			_ = ws.WriteJSON(map[string]any{
				"type":     "log",
				"batch_id": batchID,
				"message":  logLine,
			})
		}
	}

	subscription := a.tasks.SubscribeBatch(batchID)
	defer a.tasks.UnsubscribeBatch(batchID, subscription)

	_ = conn.SetReadDeadline(time.Now().Add(35 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(35 * time.Second))
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(35 * time.Second))

			var payload map[string]any
			if json.Unmarshal(message, &payload) != nil {
				continue
			}
			switch payload["type"] {
			case "cancel":
				a.tasks.CancelBatch(batchID)
				_ = ws.WriteJSON(map[string]any{
					"type":     "status",
					"batch_id": batchID,
					"status":   "cancelling",
					"message":  "取消请求已提交",
				})
			}
		}
	}()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case event := <-subscription:
			_ = ws.WriteJSON(event)
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}
}
