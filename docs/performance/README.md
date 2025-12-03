# Performance Analysis & Optimization Docs

This directory contains analysis and troubleshooting guides related to the performance of the Color Game system.

## Contents

### 1. Connection Handling
*   [Gateway Connection Analysis](gateway_analysis.md): Analyzes why the Gateway service drops connections at high concurrency (4000+ users), covering OS limits, buffer overflows, and timeouts.

### 2. CPU & Resource Usage
*   [Login CPU Spike Analysis](login_cpu_analysis.md): Investigates why the User Service/Monolith CPU spikes during mass login events.
*   [User Module Design](USER_DESIGN.md): (中文) 說明 User 模組的設計，包含 Bcrypt 密碼處理與登入接口的限流保護策略 (Rate Limiting)。

### 3. Load Testing Logs
*   [Load Test Troubleshooting](load_test_troubleshooting.md): A running log of investigations during load testing (e.g., "Connection Reset" at 4500 users), documenting steps taken and findings.

### 4. Logging Impact
*   [Logging Impact Analysis](logging_impact_analysis.md): Explains how synchronous console logging caused "Connection Reset by Peer" errors under high load.
*   [Logging Architecture](LOGGING_ARCHITECTURE.md): (中文) 詳細說明目前的日誌架構、寫入策略 (Sync/Async) 與檔案輪轉設定。
