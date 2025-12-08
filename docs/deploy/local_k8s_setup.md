# Local Kubernetes Setup Guide (Kind)

本文件記錄如何使用 [Kind (Kubernetes in Docker)](https://kind.sigs.k8s.io/) 在本地端搭建一個擬真的 3 節點 Kubernetes 集群，並進行全端微服務部署。

## 1. 什麼是 Kind?

Kind 是一個使用 Docker 容器作為 Node (節點) 來運行 K8s 集群的工具。
*   **輕量化**：不需要開虛擬機，直接跑在 Docker 上。
*   **擬真**：可以模擬多節點 (Multi-node) 環境，測試服務發現與跨節點通訊。

## 2. 配置文件說明

配置文件位於：`deploy/kind/kind-config.yaml`

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  # ... (Port Mappings for Ingress and NodePort)
- role: worker
- role: worker
```

我們配置了一个 **1 Master (Control Plane) + 2 Worker** 的架構，並預先映射了以下端口：
*   **Host 80 / 443**: Ingress Controller 入口。 
*   **Host 18848**: Nacos Console (http://localhost:18848)。

## 3. 常用指令

### 啟動集群
```bash
kind create cluster --config deploy/kind/kind-config.yaml --name game-cluster
```

### 檢查集群狀態
```bash
# 查看節點
kubectl get nodes

# 查看所有 Pods
kubectl get pods -A
```

## 4. 監控工具 (K9s)

強烈建議安裝並使用 `k9s` 來管理集群：
```bash
brew install k9s
k9s
```

## 5. 一鍵部署 (Automated Deployment)

我們提供了一個自動化腳本，可以完成從 Docker Build 到 K8s Apply 的所有流程。

### 步驟
1. 確保 Kind 集群已啟動。
2. 執行腳本：
```bash
./deploy/build_all.sh
```

**腳本會自動執行以下動作：**
1.  編譯並打包所有微服務 (User, GMS, GS, Gateway, Ops) 的 Docker Image。
2.  將 Image 載入到 Kind 節點中 (避免 Pull Image Error)。
3.  部署基礎設施 (Ingress, ConfigMap, Secret)。
4.  部署應用程式 (Deployment, Service)。

## 6. 服務訪問指南

當部署完成且 Pod 狀態為 `Running` 後，您可以在本地進行訪問。

**注意**：由於 Mac Docker 的端口綁定限制，若 80 Port 無法訪問，建議使用 Port Forward 將 Ingress 轉發到其他端口 (例如 8088)。

```bash
# 開啟 Ingress 通道 (保持此 Terminal 開啟)
kubectl port-forward -n ingress-nginx service/ingress-nginx-controller 8088:80
```

### 訪問地址 (Base URL: http://localhost:8088)

| 服務 | 說明 | 測試 URL |
| :--- | :--- | :--- |
| **Ops Center** | 運維後台 Web UI | [http://localhost:8088/ops/](http://localhost:8088/ops/) |
| **User Service** | REST API | `http://localhost:8088/api/users/health` |
| **Gateway** | WebSocket 服務 | `ws://localhost:8088/ws` |
| **Nacos Console** | 服務發現控制台 | [http://localhost:18848/nacos](http://localhost:18848/nacos) |

## 7. 常見問題

*   **Ops 頁面 404**：請確認網址結尾有 `/`，例如 `/ops/`。
*   **資料庫連線失敗**：如果是 `password authentication failed`，請參考 `docs/deploy/k8s/postgres.md` 確認 Secret 設定與 DB 實際密碼一致。
