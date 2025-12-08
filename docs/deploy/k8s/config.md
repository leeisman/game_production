# K8s Configuration Management

本文件說明微服務在 Kubernetes 環境下的配置管理策略，遵循 12-Factor App 原則與微服務的 Share-Nothing 架構。

## 1. 配置策略 (Strategy)

我們採用 **服務獨立配置 (Service-Specific Configuration)** 策略，而非單一巨大的 `common-config`。這確保了服務之間的低耦合性。

我們將配置分為兩類並「外部化 (Externalize)」：

1.  **ConfigMap (`{service}-config`)**: 非敏感數據，如服務端口、環境標籤、連接超時設定。
2.  **Secret (`{service}-secret`)**: 敏感數據，如資料庫密碼、API Token (Base64 Encoded)。

所有微服務透過 Kubernetes 的 `envFrom` 機制將這些 ConfigMap 和 Secret 注入為環境變數。

## 2. 配置清單

### User Service
*   **ConfigMap**: `deploy/k8s/config/user-config.yaml`
*   **Secret**: `deploy/k8s/config/user-secret.yaml`
    *   包含: `DB_PASSWORD`, `JWT_SECRET`, `NACOS_AUTH_TOKEN`

### Color Game Service (GMS & GS)
GMS (Game Match Service) 與 GS (Game Service) 屬於同一業務領域，共享配置。
*   **ConfigMap**: `deploy/k8s/config/colorgame-config.yaml`
*   **Secret**: `deploy/k8s/config/colorgame-secret.yaml`
    *   包含: `DB_PASSWORD`, `NACOS_AUTH_TOKEN`

### Gateway Service
*   **ConfigMap**: `deploy/k8s/config/gateway-config.yaml`
*   **Secret**: `deploy/k8s/config/gateway-secret.yaml`
    *   包含: `NACOS_AUTH_TOKEN`

## 3. 應用程式整合 (Go Integration)

微服務代碼 (`internal/config/*.go`) 透過 `os.Getenv` 讀取環境變數。

### Deployment YAML 範例
在每個服務的 Deployment 中，我們引用對應的 ConfigMap 與 Secret：

```yaml
        envFrom:
        - configMapRef:
            name: user-config
        - secretRef:
            name: user-secret
```

這能保證程式碼不需要任何修改，即可在本地 (`localhost`) 與 K8s (`Service DNS`) 之間切換。例如，本地開發時 `DB_HOST` 預設為 `localhost`，而 K8s 中 `user-config` 將其覆寫為 `postgres` (Service Name)。
