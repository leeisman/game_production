# Troubleshooting Log: Connection Reset at 4500 Users

## Problem Description
*   **Scenario**: Load testing with 4500 concurrent users.
*   **Error**: Clients receive `read: connection reset by peer`.
*   **Context**: Testing locally on macOS.

## Investigation Steps

### Step 1: Check OS Network Interface (Loopback)
*   **Hypothesis**: The OS loopback interface (`lo0`) might be dropping packets due to buffer overflows or congestion.
*   **Command**: `netstat -i | grep lo0`
*   **Output**:
    ```
    lo0        16384 <Link#1>                      47299821     0 47299857     0     0
    lo0        16384 127           localhost       47299821     - 47299857     -     -
    lo0        16384 localhost   ::1               47299821     - 47299857     -     -
    lo0        16384 mac.local   fe80:1::1         47299821     - 47299857     -     -
    ```
*   **Analysis**:
    *   The columns for `netstat -i` on macOS are typically: `Name Mtu Network Address Ipkts Ierrs Opkts Oerrs Coll`.
    *   The output shows `0` for **Ierrs** (Input Errors) and **Oerrs** (Output Errors).
    *   **Conclusion**: The loopback interface itself is NOT dropping packets or reporting hardware/driver-level errors. The network pipe is healthy at the link layer.

## Potential Causes & Next Steps

Since the interface is fine, the "Connection Reset" is likely coming from the **TCP layer** or the **Application layer**.

### 1. File Descriptor Exhaustion (High Probability)
*   **Theory**: The Gateway process hit the open file limit (default 256 or 4096 on Mac). When `Accept()` fails or cannot create a new socket, the OS might reset the connection.
*   **Action**: Check `ulimit -n` and the process's specific limit.

### 2. TCP Backlog Overflow
*   **Theory**: The application cannot `Accept()` connections fast enough. The kernel's listen queue (backlog) fills up. When full, the kernel rejects new SYN packets or completed handshakes.
*   **Action**: Check `netstat -An` or look for dropped connections in kernel stats (harder on Mac than Linux).

### 3. Application Timeout / Panic
*   **Theory**: The Gateway accepted the connection but immediately closed it because of an internal error (e.g., "Too many open files" error in logs, or a logic timeout).
*   **Action**: Check Gateway logs for `accept: too many open files` or similar errors.

### 4. Test Robot Betting Logic (Client-Side Issue)
*   **Theory**: The issue originates from the `test_robot` implementation during the betting phase.
    *   **Previous Behavior**: The robot spawned a new goroutine (`go r.PlaceBet`) for every bet immediately upon receiving the "start betting" signal.
    *   **Impact**: With 4500 users, this created a "thundering herd" of 4500 concurrent goroutines trying to write to WebSocket connections simultaneously. This could overwhelm the local network stack or cause race conditions if not thread-safe.
*   **Fix Applied**: Refactored `test_robot` to use a single event loop with a `time.Timer`.
    *   **Optimization**: Bets are now scheduled with a random delay (0-3s) and executed sequentially within the main loop, preventing burst writes and ensuring thread safety.
