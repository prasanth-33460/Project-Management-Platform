package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Event is the typed payload broadcast to clients.
type Event struct {
	Type    string      `json:"type"`    // issue_created | issue_updated | issue_moved | comment_added | sprint_updated | presence_update
	Payload interface{} `json:"payload"`
}

// PresenceInfo is broadcast when users join/leave a board.
type PresenceInfo struct {
	BoardID string      `json:"board_id"` // project_id
	Users   []uuid.UUID `json:"users"`
}

// Client represents a single WebSocket connection.
type Client struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Rooms  []string
	send   chan []byte
	hub    *Hub
}

type registration struct {
	client *Client
	rooms  []string
}

// Hub manages all WebSocket clients and pub/sub via Redis.
type Hub struct {
	mu         sync.RWMutex
	rooms      map[string]map[*Client]bool // room -> set of clients
	register   chan registration
	unregister chan *Client
	rdb        *redis.Client
}

const presenceTTL = 35 * time.Second

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan registration, 64),
		unregister: make(chan *Client, 64),
		rdb:        rdb,
	}
}

// Run starts the hub event loop and the Redis subscriber. Call this in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	pubsub := h.rdb.PSubscribe(ctx, "ws:*")
	defer pubsub.Close()

	msgCh := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return

		case reg := <-h.register:
			h.mu.Lock()
			for _, room := range reg.rooms {
				if h.rooms[room] == nil {
					h.rooms[room] = make(map[*Client]bool)
				}
				h.rooms[room][reg.client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			for _, room := range client.Rooms {
				if clients, ok := h.rooms[room]; ok {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.rooms, room)
					}
				}
			}
			h.mu.Unlock()
			close(client.send)

		case msg := <-msgCh:
			// msg.Channel is e.g. "ws:project:abc123"; strip the "ws:" prefix
			room := msg.Channel[3:]
			h.fanOut(room, []byte(msg.Payload))
		}
	}
}

func (h *Hub) fanOut(room string, data []byte) {
	h.mu.RLock()
	clients := h.rooms[room]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.send <- data:
		default:
			// slow client — drop rather than block
		}
	}
}

// Broadcast publishes an event to a room via Redis Pub/Sub.
// Room is typically "project:{id}" or "issue:{id}".
func (h *Hub) Broadcast(room string, event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.rdb.Publish(ctx, "ws:"+room, data).Err(); err != nil {
		log.Printf("ws broadcast error: %v", err)
	}
}

// SetPresence marks a user as present on a board with a TTL.
func (h *Hub) SetPresence(ctx context.Context, projectID, userID string) {
	key := "presence:board:" + projectID + ":" + userID
	h.rdb.Set(ctx, key, "1", presenceTTL)
}

// RemovePresence clears a user's presence immediately on disconnect.
func (h *Hub) RemovePresence(ctx context.Context, projectID, userID string) {
	key := "presence:board:" + projectID + ":" + userID
	h.rdb.Del(ctx, key)
}

// GetPresence returns all users currently viewing a board.
func (h *Hub) GetPresence(ctx context.Context, projectID string) []string {
	pattern := "presence:board:" + projectID + ":*"
	keys, err := h.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil
	}
	prefix := "presence:board:" + projectID + ":"
	var users []string
	for _, k := range keys {
		users = append(users, k[len(prefix):])
	}
	return users
}

func (h *Hub) newClient(userID uuid.UUID, rooms []string) *Client {
	c := &Client{
		ID:     uuid.New(),
		UserID: userID,
		Rooms:  rooms,
		send:   make(chan []byte, 256),
		hub:    h,
	}
	h.register <- registration{client: c, rooms: rooms}
	return c
}

func (h *Hub) disconnect(c *Client) {
	h.unregister <- c
}
