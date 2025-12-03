# Gateway Connection Limit Analysis

## Issue Description
The gateway service is experiencing "Connection Reset by Peer" errors when reaching approximately 4000 concurrent connections.

## Likely Causes

### 1. OS File Descriptor Limits (Most Likely)
On macOS and Linux, every network connection requires a file descriptor (FD).
*   **Default Limits**: macOS often defaults to `256` or `4096` open files per process.
*   **Symptom**: When the limit is reached, the OS refuses new connections, often resulting in a reset (RST) or timeout.
*   **Diagnosis**:
    Run the following command to check the limit for the current shell:
    ```bash
    ulimit -n
    ```
    To check the limit for a running process:
    ```bash
    lsof -p <PID> | wc -l
    ```
*   **Solution**:
    Increase the limit in your shell before starting the server:
    ```bash
    ulimit -n 65535
    ```
    Note: On macOS, you might need to set `kern.maxfiles` via `sysctl` if you need very high limits (e.g., > 10k).

### 2. TCP Backlog (Listen Queue)
When connections arrive faster than the application can `Accept()` them, they queue up in the kernel's backlog.
*   **Default Limits**: The default backlog size is often small (e.g., 128).
*   **Symptom**: If the queue fills up, the OS drops the SYN packet or sends a RST.
*   **Solution**:
    Increase the system-wide limit:
    ```bash
    sudo sysctl -w kern.ipc.somaxconn=4096  # macOS
    # or
    sudo sysctl -w net.core.somaxconn=4096  # Linux
    ```

### 3. Ephemeral Port Exhaustion
If the gateway is making outgoing connections (e.g., to Redis, User Service, or GMS) for each incoming client, it might run out of ephemeral ports.
*   **Analysis**: The current code uses shared clients (`userClient`, `gmsClient`, `rdb`), which is good. However, if there are any per-request connections being created and not closed, this could be an issue.
*   **Diagnosis**:
    Check for sockets in `TIME_WAIT` state:
    ```bash
    netstat -an | grep TIME_WAIT | wc -l
    ```

### 4. Application-Level Disconnections (Buffer Overflow)
The `ws/manager.go` implementation has strict policies for slow clients:
*   **Broadcast**: In `Broadcast()`, if a client's `Send` channel (buffer size 256) is full, the server **immediately closes the connection** (`close(client.Send)`).
    *   *Scenario*: If the game generates events faster than the client can read them, the buffer fills up, and the server drops the client.
*   **SendToUser**: Has a 5-second timeout. If it can't send within 5s, it closes the connection.

### 5. WebSocket Handshake Timeout
The `ws/manager.go` and `adapter/http/handler.go` have timeouts.
*   `WriteWait`: 10s
*   `PongWait`: 60s
*   If the server is under heavy load (CPU 100%), it might be slow to respond to the WebSocket handshake or Pings, causing the client or server to time out and close the connection.

## Code Review Findings
*   **No Explicit Application Limit**: The `ws.Manager` and `http.Handler` do not have any code that explicitly limits the number of connections (e.g., `if len(clients) > 4000 { reject }`).
*   **Resource Usage**: Each WebSocket connection spawns 2 goroutines (`ReadPump` and `WritePump`). 4000 connections = 8000 goroutines. This is generally handled well by Go, provided there is enough memory.

## Recommendations
1.  **Verify `ulimit`**: Ensure the process is actually running with a high FD limit.
2.  **Check CPU/Memory**: Monitor if the gateway is hitting CPU limits during the connection spike.
3.  **Rate Limiting**: Consider adding a rate limiter to the `HandleWebSocket` function to prevent a thundering herd of connections from overwhelming the handshake process.
4.  **Load Testing**: When testing, ensure the *client* (load tester) also has high `ulimit` and enough resources.
