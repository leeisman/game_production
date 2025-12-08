# Redis on Kubernetes (Kind) 部署指南

本文件記錄 Redis 的部署配置。目前階段 Redis 僅用於 **GMS State Machine** 的健康檢查依賴與 **Gateway** 的潛在緩存需求，暫不設定持久化 (Persistence)。

## 1. 架構與配置

*   **Image**: `redis:alpine` (輕量版)
*   **Service**: ClusterIP `redis:6379` (僅供內部微服務使用，不對外暴露)。
*   **Persistence**: None (重啟後緩存清空)。

## 2. ConfigMap 設定

位於 `deploy/k8s/config/common-config.yaml`：
```yaml
REDIS_HOST: "redis"
REDIS_PORT: "6379"
```

## 3. 連線驗證

Redis 被設計為無密碼訪問 (開發環境預設)，若未來需要密碼，請更新 `common-secret.yaml` 並修改啟動參數。

### 內部連線
```bash
kubectl exec -it <pod-name> -- redis-cli -h redis
```
