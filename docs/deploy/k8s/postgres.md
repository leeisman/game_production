# PostgreSQL on Kubernetes (Kind) 部署指南

本文件記錄如何在 Local Kind 環境下部署具有「持久化數據 (Persistence)」能力的 PostgreSQL 資料庫。

## 1. 架構設計 (Architecture)

### 1.1 資料持久化策略 (Persistence Strategy)
為了確保電腦重開機或 Kind Cluster 刪除後資料不遺失，我們採用 **HostPath Mount** 策略：

1.  **Kind 層**: 在 `kind-config.yaml` 中將 Mac 本機的 `./deploy/k8s/data` 掛載到容器內的 `/data`。
2.  **K8s 層**: 使用 `Postgres Deployment` 掛載 `/data/postgres` 到資料庫的 `/var/lib/postgresql/data`。

### 1.2 網路策略 (Networking)
*   **內部**: 使用 Service `postgres:5432` 供微服務連線。
*   **外部**: 開放 NodePort `30543` 方便 GUI 工具管理，但因未在 Kind 預留 Mapping，需使用 Port Forward 連線。

## 2. 部署配置 (Configuration)

### 2.1 Kind Configuration
必須包含 `extraMounts`:
```yaml
nodes:
- role: control-plane
  extraMounts:
  - hostPath: ./deploy/k8s/data   # Mac 本地路徑
    containerPath: /data          # 容器內路徑
```

### 2.2 StatefulSet YAML
```yaml
      volumes:
      - name: postgres-data
        hostPath:
          path: /data/postgres    # 指向上面掛載進來的路徑
          type: DirectoryOrCreate
```

## 3. 連線指南 (Connection Guide)

### 3.1 微服務連線 (Internal)
*   Host: `postgres`
*   Port: `5432`
*   User: `postgres`
*   Pass: `password` (Stored in Secret)
*   DB: `game_db`

### 3.2 Mac 本地連線 (External)
使用 `kubectl port-forward` 或 K9s `Shift-f`:
```bash
kubectl port-forward statefulset/postgres 15432:5432
```
連線字串：`postgres://postgres:password@localhost:15432/game_db`
