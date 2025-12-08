# K9s 實用指令與操作速查表 (Cheatsheet)

K9s 是一個終端機介面 (TUI) 的 Kubernetes 管理工具，類似於 VIM 的操作邏輯，能大幅提升 K8s 的維運效率。

## 1. 基礎導航 (Navigation)

進入 K9s 後，按下 `:` (冒號) 即可輸入資源類型進行跳轉：

| 指令 | 說明 | 備註 |
| :--- | :--- | :--- |
| `:pods` | 查看所有 Pods | **最常用**。顯示容器運作狀態。 |
| `:svc` | 查看 Services | 顯示 Service IP (ClusterIP) 與端口映射。 |
| `:deploy` | 查看 Deployments | 管理應用程式的部署與副本數。 |
| `:ns` | 切換 Namespace | 在不同命名空間 (如 default, kube-system) 切換。 |
| `:pulse` | 儀表板 (Dashboard) | **全覽模式**。顯示集群健康度、資源總覽。 |
| `:xray <resource>` | 透視圖 (Tree View) | 例如 `:xray pods`，以樹狀圖顯示依賴關係。 |

*   **搜尋**: 按下 `/` 可輸入關鍵字過濾列表 (例如 `/nacos`)。
*   **退出**: 按 `Ctrl-c`。

## 2. Pod 操作 (Action)

在 `:pods` 列表中選中某個 Pod 後的常用快捷鍵：

|按鍵 | 功能 | 說明 |
| :--- | :--- | :--- |
| **`l`** | **Logs (日誌)** | 即時查看容器輸出。按 `s` 可切換 wrap/unwrap，按 `ESC` 返回。 |
| **`s`** | **Shell (進入容器)** | 等同 `kubectl exec -it`。進入容器內部打指令 (如 `curl`, `netstat`)。輸入 `exit` 離開。 |
| **`d`** | **Describe (描述)** | 查看詳細狀態。**除錯必備**，查看 Events (為何起不來/Image拉不到)。 |
| **`y`** | **YAML** | 查看該資源的完整 YAML 配置。 |
| **`e`** | **Edit (編輯)** | 直接用 VIM 修改線上配置。**存檔後會立刻觸發重啟** (慎用)。 |
| **`Ctrl-d`** | **Delete (刪除)** | 殺掉該 Pod (Deployment 會自動重啟一個新的)。用於測試重啟機制。 |

## 3. 進階必殺技：Port Forward (端口轉發)

當你想從本機瀏覽器訪問 K8s 內部的服務 (且未配置 NodePort/Ingress) 時使用。

1.  **建立轉發**:
    *   在 `:pods` 選中目標。
    *   按下 **`Shift-f`**。
    *   設定對映：`Local-Port:Container-Port` (例如 `28848:8848`)。
2.  **管理轉發**:
    *   輸入 **`:pf`** 查看所有轉發通道。
    *   若 Pod 重建 (刪除/更新)，舊與的轉發會失效 (紅字/灰色)，需在這裡 `Ctrl-d` 刪除舊的再重新建立。
3.  **驗證**:
    *   本地終端機輸入 `lsof -i :<local-port>` 確認 K9s 是否在監聽。

## 4. 資源指標解讀 (Resource Columns)

在 Pod 列表中會看到 `CPU/RL` 與 `MEM/RL`：

*   **R (Request)**: **「低消」**。K8s 保證分配給容器的最小資源量。
*   **L (Limit)**: **「上限」**。容器能使用的最大資源量。
    *   **%RL**: 目前使用量佔 Limit 的百分比。
*   **顏色指標**:
    *   🟢 **綠色**: 健康，資源充足。
    *   🟡 **黃色**: 負載升高。
    *   🔴 **紅色**: **危險 (OOM Risk)**。接近 Limit 上限，記憶體爆了會被殺掉 (OOMKilled)。

## 5. 常見除錯流程 (SOP)

1.  **Pod 起不來 (`Pending`, `CrashLoopBackOff`)**:
    *   先按 `d` (Describe) 看最下面的 **Events**。
    *   如果是 `ImagePullBackOff`: 檢查 Image Name/Tag 或網路。
    *   如果是 `CrashLoopBackOff`: 按 `l` (Logs) 看程式報錯。
2.  **服務連不到**:
    *   進 Shell (`s`) 用 `curl` 自測 (Self checker)。
    *   檢查 Service (`:svc`) 的 Ports 定義是否正確對應到 Container Port。
