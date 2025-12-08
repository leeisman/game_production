# Local Kubernetes Setup Guide (Kind)

本文件記錄如何使用 [Kind (Kubernetes in Docker)](https://kind.sigs.k8s.io/) 在本地端搭建一個擬真的 3 節點 Kubernetes 集群，並進行全端微服務部署。

## 1. 什麼是 Kind?

Kind 是一個使用 Docker 容器作為 Node (節點) 來運行 K8s 集群的工具。
*   **輕量化**：不需要開虛擬機，直接跑在 Docker 上。
*   **擬真**：簡單模擬多節點 (1 Master + 2 Worker) 環境。

## 2. 配置文件說明

配置文件位於：`deploy/kind/kind-config.yaml`。
我們預先映射了以下端口，**無需手動執行 kubectl port-forward**：

*   **Host 8088**: Ingress Controller 入口 (因為 Mac 80 port 常被佔用，故改用 8088)。
*   **Host 30081**: Gateway Service NodePort (WebSocket/gRPC 直連)。
*   **Host 18848**: Nacos Console (NodePort: 30848)。
*   **Host 15432**: Postgres Database (NodePort: 30432)。

## 2.1 流量轉發機制說明 (Traffic Flow)

### Ingress 模式 (Host -> Control Plane -> Ingress Controller -> Service -> Pod)
*   **入口**: `k8s.ops.local:8088`
*   **路徑**:
    1.  流量打到 Localhost:8088。
    2.  Docker 轉發到 **Control Plane 容器** 的 80 Port。
    3.  Control Plane 上的 `ingress-nginx-controller` (HostPort 80) 接收流量。
    4.  Nginx 根據 Domain 分流到後端 Service (ClusterIP)。
*   **關鍵**: 這就是為什麼 `ingress-nginx-controller` 必須跑在 Control Plane (Label: `ingress-ready=true`)，否則外部流量進不去。

### NodePort 模式 (Host -> Control Plane -> Kube-Proxy -> Service -> Pod)
*   **入口**: `localhost:30081` (Gateway)
*   **路徑**:
    1.  流量打到 Localhost:30081。
    2.  Docker 轉發到 **Control Plane 容器** 的 30081 Port。
    3.  Control Plane 上的 `kube-proxy` 攔截流量。
    4.  轉發到任意節點上的 Pod (透過 CNI 網路)。
*   **關鍵**: NodePort 允許我們繞過 Ingress 直接連線，適合 WebSocket 或 Database 調試。

## 3. 本地 DNS 設定 (必做！)

為了讓 Ingress 正確分流流量，請修改 Mac 的 `/etc/hosts` 文件，加入以下域名：

```bash
# 執行以下指令加入設定
sudo sh -c 'echo "127.0.0.1 k8s.ops.local k8s.user.local k8s.gateway.local" >> /etc/hosts'
```

## 4. 環境初始化 (First Time Setup)

由於我們移除了自動化腳本，請依序執行以下指令來完成環境建置：

### 步驟 1: 建立 Cluster
```bash
# 刪除舊的 (如果有)
kind delete cluster --name game-cluster

# 建立新的 (使用自定義 config)
kind create cluster --name game-cluster --config deploy/kind/kind-config.yaml
```

### 步驟 2: 安裝 Ingress Controller
```bash
# 下載並安裝官方 Manifest
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# 等待 Pod 就緒
kubectl wait --namespace ingress-nginx --for=condition=available deployment/ingress-nginx-controller --timeout=300s

# [關鍵] Patch Controller: 綁定到 Control Plane 並移除資源限制
kubectl patch deployment ingress-nginx-controller -n ingress-nginx -p '{"spec": {"template": {"spec": {"nodeSelector": {"ingress-ready": "true"}}}}}'
kubectl patch deployment ingress-nginx-controller -n ingress-nginx --type json -p='[{"op": "remove", "path": "/spec/template/spec/containers/0/resources/limits"}]'
```

### 步驟 3: 部署基礎設施 (Infrastructure)
```bash
# ConfigMaps & Secrets
kubectl apply -f deploy/k8s/config/

# Metrics Server (for k9s/kubectl top)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

# Services
kubectl apply -f deploy/k8s/nacos.yaml
kubectl apply -f deploy/k8s/redis.yaml
kubectl apply -f deploy/k8s/postgres.yaml
```

### 步驟 4: 建置與部署應用程式 (Applications)
執行我們提供的建置腳本：
```bash
./deploy/build_all.sh
```
此腳本會：
1. 編譯 Go 微服務。
2. 建置 Docker Images 並 Load 進 Kind。
3. 執行 `kubectl apply -f deploy/k8s/app/`。
4. 滾動更新 Deployment。

## 5. 服務訪問指南

當 Pod 狀態為 `Running` 後，可直接通過瀏覽器訪問：

| 服務 | 域名 (Domain) | 測試 URL | 備註 |
| :--- | :--- | :--- | :--- |
| **Ops Center** | `k8s.ops.local` | [http://k8s.ops.local:8088/](http://k8s.ops.local:8088/) | 運維後台 Web UI |
| **User Service** | `k8s.user.local` | `http://k8s.user.local:8088/api/users/health` | REST API |
| **Gateway (Ingress)** | `k8s.gateway.local` | `ws://k8s.gateway.local:8088/ws` | WebSocket via Ingress |
| **Gateway (Direct)** | `localhost` | `ws://localhost:30081/ws` | WebSocket via NodePort |
| **Nacos** | `localhost` | [http://localhost:18848/nacos](http://localhost:18848/nacos) | 帳號密碼預設: nacos/nacos |

## 6. 資料庫連線

本地開發工具 (如 Navicat, Datagrip) 可直接連線：
*   **Host**: `localhost`
*   **Port**: `15432` (我們在 kind-config 映射了 Host 15432 -> NodePort 30432)
*   **User**: `postgres`
*   **Password**: `password`
*   **Database**: `game_db`

*注意：`deploy/k8s/data` 目錄用於持久化數據，請勿隨意刪除。*

## 7. 監控與除錯

*   使用 `k9s` 管理集群。
*   若 Ingress 不通，請檢查 `kubectl get pods -n ingress-nginx` 是否 Running。
*   若 Ops 頁面 404，請確認網址是否包含 Port `8088` 且域名正確。

## 8. 常見問題與故障排除 (Troubleshooting)

### Q1: 瀏覽器或 Postman 無法連線 (Connection Refused / Gateway Timeout)，但內部 Curl 正常？

**症狀**：
*   訪問 `http://k8s.ops.local:8088` 失敗。
*   檢查 `docker ps` 發現 Port 8088 確實有開啟。

**原因**：
*   Kind 的 `extraPortMappings` (我們用來開 8088 的設定) **只綁定在 Control Plane 節點**。
*   如果 Kubernetes 調度器將 `ingress-nginx-controller` Pod 分配到了 **Worker Node**，外部流量進入 Control Plane 後找不到 Ingress，導致連線失敗。

**解決方案**：
強制將 Ingress Controller 綁定到 Control Plane 節點（因為只有 Control Plane 有開 Host Port 映射）。執行以下 Patch：

```bash
kubectl patch deployment ingress-nginx-controller -n ingress-nginx -p '{"spec": {"template": {"spec": {"nodeSelector": {"ingress-ready": "true"}}}}}'
```

---
