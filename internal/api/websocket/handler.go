package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	gows "github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

const (
	readTimeout  = 60 * time.Second
	writeTimeout = 10 * time.Second
)

// Upgrade gates the WebSocket upgrade. Must come before gows.New().
func Upgrade(c *fiber.Ctx) error {
	if gows.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

// Handler drives individual WebSocket connections.
type Handler struct{ hub *Hub }

func NewHandler(hub *Hub) *Handler { return &Handler{hub: hub} }

// Handle is the gofiber/websocket entry point.
// Query params: user_id (required), project_id, issue_id (at least one room required).
func (h *Handler) Handle(c *gows.Conn) {
	userIDStr := c.Query("user_id")
	projectID := c.Query("project_id")
	issueID := c.Query("issue_id")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		if wErr := c.WriteJSON(fiber.Map{"error": "invalid user_id"}); wErr != nil {
			slog.Warn("ws: write error reject", "error", wErr)
		}
		return
	}

	var rooms []string
	if projectID != "" {
		rooms = append(rooms, "project:"+projectID)
	}
	if issueID != "" {
		rooms = append(rooms, "issue:"+issueID)
	}
	if len(rooms) == 0 {
		if wErr := c.WriteJSON(fiber.Map{"error": "provide project_id or issue_id query param"}); wErr != nil {
			slog.Warn("ws: write error reject", "error", wErr)
		}
		return
	}

	client := h.hub.newClient(userID, rooms)
	defer h.hub.disconnect(client)

	ctx := context.Background()

	if projectID != "" {
		h.hub.SetPresence(ctx, projectID, userIDStr)
		defer h.hub.RemovePresence(ctx, projectID, userIDStr)
		h.broadcastPresence(projectID)
	}

	// write pump: drains client.send onto the connection
	go func() {
		for msg := range client.send {
			if err := c.WriteMessage(1, msg); err != nil {
				slog.Debug("ws: write pump error", "user_id", userIDStr, "error", err)
				return
			}
		}
	}()

	// read pump: handles heartbeats and extends deadlines
	if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		slog.Warn("ws: set read deadline failed", "user_id", userIDStr, "error", err)
	}

	c.SetPongHandler(func(string) error {
		if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			slog.Warn("ws: pong deadline renew failed", "user_id", userIDStr, "error", err)
		}
		if projectID != "" {
			h.hub.SetPresence(ctx, projectID, userIDStr)
		}
		return nil
	})

	for {
		msgType, msg, err := c.ReadMessage()
		if err != nil {
			break // normal close or network error
		}
		if msgType != 1 {
			continue // ignore binary frames
		}

		var incoming map[string]interface{}
		if err := json.Unmarshal(msg, &incoming); err != nil {
			continue
		}

		if incoming["type"] == "heartbeat" {
			if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				slog.Warn("ws: heartbeat deadline renew failed", "user_id", userIDStr, "error", err)
			}
			if projectID != "" {
				h.hub.SetPresence(ctx, projectID, userIDStr)
			}
			if wErr := c.WriteJSON(fiber.Map{"type": "heartbeat_ack"}); wErr != nil {
				slog.Debug("ws: heartbeat_ack write failed", "user_id", userIDStr, "error", wErr)
			}
		}
	}

	slog.Debug("ws: client disconnected", "user_id", userIDStr)
	if projectID != "" {
		h.broadcastPresence(projectID)
	}
}

func (h *Handler) broadcastPresence(projectID string) {
	users := h.hub.GetPresence(context.Background(), projectID)
	h.hub.Broadcast("project:"+projectID, &Event{
		Type: "presence_update",
		Payload: map[string]interface{}{
			"board_id": projectID,
			"users":    users,
		},
	})
}
