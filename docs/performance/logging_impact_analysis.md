# Logging Impact Analysis: Why "Debug" Level Caused Connection Resets

## Observation
Changing the log level from `debug` to `error` in `monolith/main.go` resolved the `read tcp connection reset by peer` errors during load testing (4500 users).

## Root Cause Analysis

The issue is likely caused by **Synchronous I/O Blocking** and **CPU Saturation** due to excessive logging.

### 1. Synchronous Console Output (The Bottleneck)
*   **Configuration**: The application is configured to use `Format: "console"`.
*   **Mechanism**: `zerolog.ConsoleWriter` performs pretty-printing (colorizing, formatting) and writes to `os.Stdout`.
*   **Blocking**: Writing to `stdout` is typically a **synchronous** and **blocking** operation.
*   **Scenario**:
    *   With 4500 users, if every message (ping, pong, broadcast, bet) generates a `Debug` log, the system tries to write thousands of lines per second to the terminal.
    *   The terminal (or the pipe connected to it) has a limited buffer. When it fills up, the `Write` syscall blocks the Go runtime.
    *   **Result**: The goroutines responsible for handling WebSocket traffic (`ReadPump`, `WritePump`) get stuck waiting for the log to be written. They cannot read from or write to the TCP socket in time.

### 2. TCP Buffer Overflow
*   Because the application goroutines are blocked on logging, they stop reading from the OS's TCP receive buffer.
*   The OS TCP buffer fills up.
*   When the client tries to send more data (e.g., a heartbeat or bet), the OS rejects the packet because the buffer is full, or the connection times out.
*   Eventually, the OS or the client resets the connection (`RST`).

### 3. CPU Overhead
*   Formatting strings, allocating memory for log fields, and colorizing output consumes significant CPU cycles.
*   In a high-concurrency test, this "logging overhead" competes with the actual business logic (handling bets, broadcasting) for CPU time.

## Conclusion
This is a classic "Observer Effect" where the act of observing the system (logging) changes its behavior. The `debug` level logging was too heavy for the system to handle at 4500 concurrent connections, causing the very timeouts and resets you were trying to debug.

## Recommendations

1.  **Production Settings**: Always use `Level: "info"` (or "warn") and `Format: "json"` in production or load testing. JSON is much faster to generate than console output.
2.  **Async Logging**: If you absolutely need high-volume logs, consider using an asynchronous logger (buffered writer) so the application doesn't block on I/O.
3.  **Sampling**: Only log a percentage of requests if you need debug info under load.
