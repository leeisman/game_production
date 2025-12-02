package ws

import (
	"context"
	"sync"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gorilla/websocket"
)

// Connection represents a WebSocket connection
type Connection struct {
	UserID    int64
	Conn      *websocket.Conn
	Send      chan []byte
	manager   *Manager
	closeOnce sync.Once
}

// Manager manages all WebSocket connections
type Manager struct {
	clients    map[int64]*Connection
	register   chan *Connection
	unregister chan *Connection
	mu         sync.RWMutex
}

// NewManager creates a new connection manager
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[int64]*Connection),
		register:   make(chan *Connection),
		unregister: make(chan *Connection),
	}
}

// Register registers a new connection
func (m *Manager) Register(conn *websocket.Conn, userID int64) *Connection {
	c := &Connection{
		UserID:  userID,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		manager: m,
	}
	m.register <- c
	return c
}

// Run starts the manager loop
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			// If user already connected, close old connection
			if old, ok := m.clients[client.UserID]; ok {
				old.Close()
			}
			m.clients[client.UserID] = client
			m.mu.Unlock()

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.UserID]; ok {
				delete(m.clients, client.UserID)
				client.Close()
			}
			m.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all connected local clients
func (m *Manager) Broadcast(message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			// We can't delete here because we hold RLock
			// The unregister channel will handle cleanup eventually
			// or we could launch a goroutine to unregister
		}
	}
}

// SendToUser sends a message to a specific user
func (m *Manager) SendToUser(userID int64, message []byte) {
	m.mu.RLock()
	client, ok := m.clients[userID]
	m.mu.RUnlock()

	if ok {
		select {
		case client.Send <- message:
			return
		default:
			// Buffer full, try to wait a bit
		}

		// Wait with timeout
		select {
		case client.Send <- message:
			return
		case <-time.After(time.Second * 5):
			// Timeout, client is too slow. Close connection to avoid blocking server.
			// We use a background context here as we don't have the request context
			logger.Warn(context.Background()).Int64("user_id", userID).Msg("SendToUser: buffer full and timed out, closing connection")
			client.Close()
		}
	}
}

// Shutdown closes all connections
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
}

// Close closes the connection
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.Send)
		c.Conn.Close()
	})
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second) // Ping period
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Connection) ReadPump(handleMessage func(int64, []byte)) {
	defer func() {
		c.manager.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(4096)                                // Max message size
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)) // Pong wait
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error
			}
			break
		}

		// Handle message
		handleMessage(c.UserID, message)
	}
}
