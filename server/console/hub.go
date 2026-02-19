package console

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type KVStore interface {
	Get(key string) (string, bool)
	Delete(keys ...string) int
	Keys() []string
	Len() int
	Set(key, value string)
}

type ServerStats interface {
	GetConnectionCount() int32
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	kv         KVStore
	server     ServerStats
	mu         sync.RWMutex
}

type StatsMessage struct {
	Type        string   `json:"type"`
	Connections int      `json:"connections"`
	KVCount     int      `json:"kv_count"`
	Keys        []string `json:"keys,omitempty"`
	Timestamp   int64    `json:"timestamp"`
}

type KVMessage struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewHub(kv KVStore, server ServerStats) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		kv:         kv,
		server:     server,
	}
}

func (h *Hub) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.sendStats(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			h.BroadcastStats()
		}
	}
}

func (h *Hub) BroadcastStats() {
	connections := 0
	if h.server != nil {
		connections = int(h.server.GetConnectionCount())
	}

	kvCount := 0
	if h.kv != nil {
		kvCount = h.kv.Len()
	}

	stats := StatsMessage{
		Type:        "stats",
		Connections: connections,
		KVCount:     kvCount,
		Timestamp:   time.Now().UnixMilli(),
	}

	data, err := json.Marshal(stats)
	if err != nil {
		log.Printf("Error marshaling stats: %v", err)
		return
	}

	h.broadcast <- data
}

func (h *Hub) sendStats(conn *websocket.Conn) {
	connections := 0
	if h.server != nil {
		connections = int(h.server.GetConnectionCount())
	}

	kvCount := 0
	if h.kv != nil {
		kvCount = h.kv.Len()
	}

	stats := StatsMessage{
		Type:        "stats",
		Connections: connections,
		KVCount:     kvCount,
		Timestamp:   time.Now().UnixMilli(),
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return
	}

	conn.WriteMessage(websocket.TextMessage, data)
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	h.register <- conn

	defer func() {
		h.unregister <- conn
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg KVMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		if h.kv == nil {
			continue
		}

		switch msg.Type {
		case "get_keys":
			keys := h.kv.Keys()
			keyData := make([]string, 0, len(keys))
			for _, k := range keys {
				if v, ok := h.kv.Get(k); ok {
					keyData = append(keyData, k+":"+v)
				}
			}
			response := StatsMessage{
				Type:      "keys",
				Keys:      keyData,
				Timestamp: time.Now().UnixMilli(),
			}
			data, _ := json.Marshal(response)
			conn.WriteMessage(websocket.TextMessage, data)
		case "set":
			h.kv.Set(msg.Key, msg.Value)
			h.BroadcastStats()
		case "delete":
			h.kv.Delete(msg.Key)
			h.BroadcastStats()
		}
	}
}

func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
