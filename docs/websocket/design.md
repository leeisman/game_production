# WebSocket Design & Implementation

This document details the architecture, design decisions, and implementation of the WebSocket layer in the Gateway service.

## 1. Core Architecture

The WebSocket implementation follows a standard **Hub-Client** pattern (managed by `Manager` and `Connection` structs).

*   **Manager**: The central hub that maintains the list of active connections and handles broadcasting.
*   **Connection**: Represents a single user's WebSocket connection, managing its own read/write loops.

---

## 2. Connection Lifecycle Management

### `Manager.Run()`
*   **Purpose**: The central event loop for managing the lifecycle of connections.
*   **Strategy**: **Single-Threaded State Mutation**.
*   **Logic**:
    *   Listens on `register` and `unregister` channels.
    *   **Register**: Safely adds a new client to the map. If a user ID already exists (duplicate login), it disconnects the *old* connection (`ReasonReplaced`) to allow the new one.
    *   **Unregister**: Safely removes a client from the map.
    *   **Reasoning**: By serializing map modifications in a single goroutine (or protecting with Mutex as implemented), we ensure thread safety without complex locking logic scattered everywhere.

```go
func (m *Manager) Run() {
    for {
        select {
        case client := <-m.register:
            m.mu.Lock()
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
```

---

## 3. I/O Handling (The Pumps)

Each connection spawns two dedicated goroutines: one for reading and one for writing. This decouples input from output.

### `Connection.ReadPump(handleMessage)`
*   **Purpose**: The dedicated goroutine for reading data *from* the WebSocket TCP socket.
*   **Strategy**: **Pull with Limit**.
*   **Logic**:
    *   **Message Loop**: continuously calls `ReadMessage()`.
    *   **Size Limit**: Enforces a max message size (4096 bytes) to prevent memory exhaustion attacks.
    *   **Pong Handler**: Resets the read deadline whenever a `Pong` is received, confirming the client is still alive.
    *   **Callback**: Passes valid messages to the `handleMessage` callback for business logic processing.
    *   **Cleanup**: On exit (error or close), it notifies the Manager to unregister the client.

```go
func (c *Connection) ReadPump(handleMessage func(int64, []byte)) {
    var readErr error
    defer func() {
        c.manager.unregister <- c
        c.CloseWithReason(ReasonReadError, readErr)
    }()

    c.Conn.SetReadLimit(4096)
    c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
        handleMessage(c.UserID, message)
    }
}
```

### `Connection.WritePump()`
*   **Purpose**: The dedicated goroutine for writing data *to* the WebSocket TCP socket.
*   **Strategy**: **Push with Heartbeat**.
*   **Logic**:
    *   **Message Loop**: Reads from the `Send` channel and writes to the WebSocket.
    *   **Ping/Pong**: Uses a `Ticker` to send `Ping` frames periodically (every 54s) to keep the connection alive through proxies/load balancers.
    *   **Deadlines**: Sets write deadlines (30s) to prevent the goroutine from hanging forever if the network cable is unplugged.
    *   **Termination**: If the `Send` channel is closed (though we removed explicit close to avoid panic) or a write error occurs, it closes the connection.

```go
func (c *Connection) WritePump() {
    ticker := time.NewTicker(54 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case message, ok := <-c.Send:
            c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
            if !ok {
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
```

---

## 4. Message Delivery Strategies

### `Manager.SendToUser(userID, message)`
*   **Purpose**: Sends a message to a specific user.
*   **Strategy**: **Non-Blocking with Fallback Timeout**.
*   **Design Goals**:
    1.  **Non-Blocking Happy Path**: In normal operation, sending a message should be instantaneous.
    2.  **Backpressure Handling**: If a client is slow, do not block the server.
    3.  **Graceful Degradation**: Disconnect clients that are too slow.

*   **Logic Breakdown**:
    1.  **Fast Path**: Try non-blocking send (`select default`). If buffer has space, return immediately.
    2.  **Slow Path**: If buffer is full, wait up to 5 seconds.
    3.  **Failure**: If timeout occurs, disconnect the client (`ReasonTimeout`).

```go
func (m *Manager) SendToUser(userID int64, message []byte) {
    m.mu.RLock()
    client, ok := m.clients[userID]
    m.mu.RUnlock()

    if ok {
        // 1. Fast Path: Non-Blocking Send
        select {
        case client.Send <- message:
            return // Success!
        default:
            // Buffer is full. Do not block here immediately.
        }

        // 2. Slow Path: Wait with Timeout
        select {
        case client.Send <- message:
            return // Success (after waiting)
        case <-time.After(time.Second * 5):
            // 3. Failure: Client is too slow
            client.CloseWithReason(ReasonTimeout, nil)
        }
    }
}
```

### `Manager.Broadcast(message []byte)`
*   **Purpose**: Efficiently sends a message to **all** connected clients.
*   **Strategy**: **Fail-Fast**.
*   **Logic**:
    *   Iterates through all clients under a Read Lock.
    *   Attempts a non-blocking send to each client's channel.
    *   **Crucial Decision**: If a client's buffer is full, we **drop the client immediately** (`CloseWithReason(ReasonBufferFull)`).
    *   **Reasoning**: Broadcasting is usually for real-time game state. If a client is lagging so much that their buffer is full, they are already desynchronized. Waiting for them would delay the update for *everyone else* (head-of-line blocking). It's better to cut the slow tail to preserve the health of the herd.

```go
func (m *Manager) Broadcast(message []byte) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, client := range m.clients {
        select {
        case client.Send <- message:
        default:
            // Buffer full, drop client immediately
            client.CloseWithReason(ReasonBufferFull, nil)
        }
    }
}
```

---

## 5. Error Handling & Cleanup

### `Connection.CloseWithReason(reason, error)`
*   **Purpose**: Centralized cleanup and logging.
*   **Strategy**: **Idempotent & Observable**.
*   **Logic**:
    *   Uses `sync.Once` to ensure the cleanup logic runs exactly once, even if called multiple times.
    *   **Logging**: Logs the *specific reason* for disconnection (e.g., `buffer_full`, `timeout`, `write_error`), which is invaluable for debugging production issues.
    *   **Action**: Closes the underlying TCP connection. (Note: We deliberately do *not* close the `Send` channel to avoid panics in concurrent senders).

```go
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
```
