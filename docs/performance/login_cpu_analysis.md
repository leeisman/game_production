# Login CPU Spike Analysis

## Issue Description
During the login phase of the load test (4000+ concurrent users), the CPU usage of the User Service (or Monolith) spikes significantly.

## Root Cause Analysis

### 1. Bcrypt Password Hashing (Primary Cause)
The most significant CPU consumer in the login/registration process is the **Bcrypt** algorithm used for password hashing and verification.

*   **Location**: `internal/modules/user/usecase/user_uc.go`
    *   **Registration**: Line 70: `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)`
    *   **Login**: Line 105: `bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))`
*   **Impact**: Bcrypt is *designed* to be slow and CPU-intensive to resist brute-force attacks.
    *   `bcrypt.DefaultCost` is usually 10, which means $2^{10}$ iterations.
    *   For 4000 users logging in, the server must perform 4000 expensive hashing operations.
    *   If users are registering for the first time (as the robot does), it performs **2** operations per user (1 Hash for Register + 1 Compare for Login).
    *   **Total Operations**: 4000 users * 2 ops = 8000 Bcrypt operations.
    *   **Rate**: The robot spawns a new user every 10ms (100 users/sec). This demands ~100-200 Bcrypt ops/sec, which can easily saturate a single CPU core or even multiple cores.

### 2. High Concurrency of Short-Lived Requests
*   **Robot Behavior**: The robot spawns users rapidly (10ms interval).
*   **Impact**: Even outside of Bcrypt, handling 100 requests/second involves:
    *   HTTP Request parsing (Gin)
    *   JSON Unmarshalling
    *   Database Queries (Check Username, Check Email, Insert User, Create Session)
    *   JWT Token Generation (Signing)
    *   Garbage Collection (Go runtime working hard to clean up short-lived objects)

### 3. Database Connection Establishment
*   Each login involves multiple DB queries.
*   If connection pooling is not tuned, opening/closing TCP connections to Postgres adds CPU overhead.

## Recommendations

### Short Term (For Load Testing)
1.  **Reduce Bcrypt Cost**: For testing purposes only, reduce the Bcrypt cost to the minimum (`bcrypt.MinCost` = 4). This will make hashing $2^{10-4} = 64$ times faster.
2.  **Slow Down Robot**: Increase the sleep interval between spawning robots (e.g., from 10ms to 20ms or 50ms) to spread the CPU load over a longer period.
3.  **Pre-seed Users**: Instead of having robots register on the fly, pre-insert 4000 users into the database. This eliminates the Registration step (saving 1 Bcrypt op per user).

### Long Term (Production)
1.  **Horizontal Scaling**: User Service is stateless. Deploy multiple replicas of the User Service behind a load balancer to distribute the CPU load of password hashing.
2.  **Rate Limiting**: Implement strict rate limiting on Login/Register endpoints to prevent attackers (or thundering herds) from exhausting CPU.
3.  **Separate Auth Service**: Keep the Authentication Service separate (as you have done) so that high CPU usage during login doesn't affect the Game Service latency.
