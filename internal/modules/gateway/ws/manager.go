package ws

import (
	"context"
	"sync"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gorilla/websocket"
)

type CloseReason string

const (
	ReasonWriteError     CloseReason = "write_error"
	ReasonPingError      CloseReason = "ping_error"
	ReasonReadError      CloseReason = "read_error"
	ReasonSendChanClosed CloseReason = "send_channel_closed"
	ReasonReplaced       CloseReason = "replaced_by_new_connection"
	ReasonShutdown       CloseReason = "server_shutdown"
	ReasonBufferFull     CloseReason = "buffer_full"
	ReasonTimeout        CloseReason = "timeout"
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
		Send:    make(chan []byte, 1024),
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
				old.CloseWithReason(ReasonReplaced, nil)
			}
			m.clients[client.UserID] = client
			m.mu.Unlock()

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.UserID]; ok {
				delete(m.clients, client.UserID)
				client.CloseWithReason(ReasonShutdown, nil)
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
			// Buffer full, drop client
			client.CloseWithReason(ReasonBufferFull, nil)
			// We can't delete here because we hold RLock
			// The unregister channel will handle cleanup eventually
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
			client.CloseWithReason(ReasonTimeout, nil)
		}
	}
}

// Shutdown closes all connections
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.CloseWithReason(ReasonShutdown, nil)
	}
}

// CloseWithReason closes the connection with a reason
func (c *Connection) CloseWithReason(r CloseReason, err error) {
	c.closeOnce.Do(func() {
		logger.Error(context.Background()).
			Int64("user_id", c.UserID).
			Str("reason", string(r)).
			Err(err).
			Msg("ws connection closed")
		// close(c.Send) // Removed to avoid panic on concurrent send
		c.Conn.Close()
	})
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Connection) WritePump() {
	ticker := time.NewTicker(54 * time.Second) // Ping period
	defer func() {
		ticker.Stop()
		// c.Conn.Close() // Removed, handled by CloseWithReason
	}()

	for {
		select {
		case message, ok := <-c.Send:
			// c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			// 優化讓timeout更大，但必須做因為防止attack server（故意不讀）
			c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.CloseWithReason(ReasonWriteError, err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				c.CloseWithReason(ReasonWriteError, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.CloseWithReason(ReasonPingError, err)
				return
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Connection) ReadPump(handleMessage func(int64, []byte)) {
	var readErr error
	defer func() {
		c.manager.unregister <- c
		c.CloseWithReason(ReasonReadError, readErr)
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
				readErr = err
			}
			break
		}

		// Handle message
		handleMessage(c.UserID, message)
	}
}
