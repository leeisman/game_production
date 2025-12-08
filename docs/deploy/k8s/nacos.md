# Nacos on Kubernetes (Kind) 部署指南

本文件記錄在本地 Kind Kubernetes 集群上部署 Nacos 的詳細配置與踩坑紀錄。

## 1. 最終成功配置 (The Working Solution)

要讓 Nacos 在 Apple Silicon (M1/M2/M3) 的 Kind 環境下正常運作，必須注意以下三點：
1.  **Image**: 使用 `nacos/nacos-server:latest` (v2.3+)，因為它才有多架構支援 (ARM64)。
2.  **Port**: 新版 Nacos (v2.x late) 將主要服務端口與 Console 端口分離，容器內部預設使用 **8080** 作為 HTTP / Console 端口，而非傳統的 8848。
3.  **Auth**: 新版強制要求設置 `NACOS_AUTH_TOKEN` 等環境變數，即使 `AUTH_ENABLE=false`。

### Deployment YAML 關鍵片段
```yaml
    spec:
      containers:
      - name: nacos
        image: nacos/nacos-server:latest  # 必須是 latest (or > v2.3.0) for ARM64
        env:
        - name: MODE
          value: "standalone"
        - name: NACOS_AUTH_ENABLE
          value: "false"
        # 必須設置以下 Auth 變數，否則啟動會報錯退出
        - name: NACOS_AUTH_IDENTITY_KEY
          value: "serverIdentity"
        - name: NACOS_AUTH_IDENTITY_VALUE
          value: "security"
        - name: NACOS_AUTH_TOKEN
          value: "SecretKey012345678901234567890123456789012345678901234567890123456789"
        ports:
        - containerPort: 8080 # 重點：容器內部是 8080
```

## 2. 端口映射策略 (Port Mapping Strategy)

在 `nacos.yaml` 中，我們採用了 **Service Port (8848)** -> **Target Port (8080)** 的映射策略：

```yaml
kind: Service
metadata: 
  name: nacos
spec:
  ports:
    - port: 8848          # Service 暴露給 K8s 內部的端口 (維持慣例)
      targetPort: 8080    # 轉發目標：轉發到容器內部的 8080
```

### 為什麼這樣設計？
1.  **維持慣例 (Convention)**: Nacos 的招牌端口是 `8848`。保留 Service Port 為 `8848` 可以讓其他微服務的配置檔維持直觀 (設定為 `nacos:8848`)，符合開發者的肌肉記憶。
2.  **解耦 (Decoupling)**: 雖然新版容器內部實作變更為 `8080`，但透過 K8s Service 層的轉發，我們屏蔽了這個底層變更。即使未來容器端口由 8080 改為 9999，我們的微服務 Client 端配置都不需要修改，只需更新 Service yaml 即可。

## 3. 踩坑紀錄 (Troubleshooting Log)

### Q1: `ErrImagePull` / `no match for platform in manifest`
*   **現象**: Pod 狀態一直是 `ImagePullBackOff`。
*   **原因**: 使用 `v2.2.3` 或更舊的 tag，官方 Docker Hub 上這些 tag 只有 AMD64 架構，沒有 ARM64。
*   **解法**: 使用 `latest` tag，它包含了 multi-arch manifest。

### Q2: 啟動後立刻 Crash (`env NACOS_AUTH_TOKEN must be set`)
*   **現象**: Pod 狀態 `CrashLoopBackOff`，Log 顯示 `env NACOS_AUTH_TOKEN must be set with Base64 String`。
*   **原因**: Nacos v2.2+ 加強安全性，移除了預設的空 Token 允許，強制要求注入 Token 環境變數。
*   **解法**: 在 YAML `env` 中補上 `NACOS_AUTH_TOKEN` (Base64 String)。

### Q3: 端口 8848 無法訪問 Console (`Connection refused` or `default port is 8080`)
*   **現象**: 
    *   訪問 `http://localhost:8848/nacos` 顯示文字 `"Nacos Console default port is 8080"`。
    *   或者 curl `127.0.0.1:8848` 直接 Refused。
*   **原因**: Nacos 架構變更，將某些端口重新分配。雖然官方文檔建議可以用環境變數改回 8848，但在 Docker 容器中似乎有相容性問題 (Crash 或無效)。
*   **解法**: **打不過就加入**。直接將 K8s Service 的 TargetPort 指向容器的 **8080**。

### Q4: K9s Port Forward 失敗
*   **現象**: 在 K9s 使用 `Shift-f` 轉發後仍無法訪問。
*   **原因**: 舊的 Pod 被刪除後，K9s 的轉發通道不會自動更新目標 IP。
*   **解法**: 刪除舊的 Port Forward (`:pf` -> `Ctrl-d`)，選中新的 Pod 重新建立。

## 3. 驗證方式

### 檢查 Pod 狀態
```bash
kubectl get pods -l app=nacos
# 應顯示 1/1 Running
```

### 內部連通性測試 (Exec)
```bash
kubectl exec -it <pod-name> -- curl -v http://localhost:8080/nacos/index.html
# 應回傳 HTTP 200 及 HTML 內容
```

### 外部訪問 (Localhost)
確保 K8s Service 配置了 `NodePort` 且 Kind 有做 Port Mapping，或使用 K9s Port Forward 到 8080。
URL: `http://localhost:<mapped-port>/nacos/index.html`
