# Project Chimera 服務間交互圖

## 系統架構圖

```mermaid
graph TB
    subgraph "前端層"
        UI[S12 Web UI / API Gateway<br/>:8092]
    end
    
    subgraph "業務邏輯層"
        S2[S2 Feature Generator<br/>:8082]
        S3[S3 Strategy Engine<br/>:8083]
        S6[S6 Position Manager<br/>:8086]
        S7[S7 Label Backfill<br/>:8087]
        S8[S8 Autopsy Generator<br/>:8088]
        S9[S9 Hypothesis Orchestrator<br/>:8089]
    end
    
    subgraph "執行層"
        S1[S1 Exchange Connectors<br/>:8081]
        S4[S4 Order Router<br/>:8084]
        S5[S5 Reconciler<br/>:8085]
    end
    
    subgraph "基礎設施層"
        S10[S10 Config Service<br/>:8090]
        S11[S11 Metrics & Health<br/>:8091]
    end
    
    subgraph "外部系統"
        BINANCE[幣安交易所]
        REDIS[Redis<br/>事件流/緩存]
        ARANGO[ArangoDB<br/>數據存儲]
        MINIO[MinIO<br/>文件存儲]
    end
    
    %% 前端到業務邏輯層
    UI -->|POST /features/recompute| S2
    UI -->|POST /decide| S3
    UI -->|POST /positions/manage| S6
    UI -->|POST /labels/backfill| S7
    UI -->|POST /autopsy/{trade_id}| S8
    UI -->|POST /experiments/run| S9
    
    %% 前端到基礎設施層
    UI -->|POST /bundles, /simulate, /promote| S10
    UI -->|GET /metrics, /alerts| S11
    
    %% 前端到執行層
    UI -->|POST /xchg/treasury/transfer| S1
    UI -->|POST /cancel| S4
    UI -->|POST /reconcile| S5
    
    %% 業務邏輯層內部交互
    S2 -->|signals:new| S3
    S3 -->|POST /orders| S4
    S6 -->|POST /orders| S4
    S6 -->|POST /xchg/treasury/transfer| S1
    
    %% 執行層內部交互
    S5 -->|POST /cancel| S4
    
    %% 配置分發
    S10 -->|cfg:events| S2
    S10 -->|cfg:events| S3
    S10 -->|cfg:events| S4
    S10 -->|cfg:events| S5
    S10 -->|cfg:events| S6
    S10 -->|cfg:events| S12
    
    %% 外部系統連接
    S1 -->|API 調用| BINANCE
    S4 -->|API 調用| BINANCE
    
    %% 數據存儲連接
    S2 --> REDIS
    S3 --> REDIS
    S4 --> REDIS
    S5 --> REDIS
    S6 --> REDIS
    S7 --> REDIS
    S8 --> REDIS
    S9 --> REDIS
    S10 --> REDIS
    S11 --> REDIS
    S12 --> REDIS
    
    S2 --> ARANGO
    S3 --> ARANGO
    S4 --> ARANGO
    S5 --> ARANGO
    S6 --> ARANGO
    S7 --> ARANGO
    S8 --> ARANGO
    S9 --> ARANGO
    S10 --> ARANGO
    S11 --> ARANGO
    S12 --> ARANGO
    
    S8 --> MINIO
```

## 交易決策流程圖

```mermaid
sequenceDiagram
    participant S2 as S2 Feature Generator
    participant S3 as S3 Strategy Engine
    participant S4 as S4 Order Router
    participant S1 as S1 Exchange Connectors
    participant S6 as S6 Position Manager
    participant BINANCE as 幣安交易所
    
    Note over S2,BINANCE: 交易決策完整流程
    
    S2->>S2: 計算特徵
    S2->>S3: signals:new 事件
    S3->>S3: 策略決策
    S3->>S4: POST /orders
    S4->>S1: 內部訂單處理
    S1->>BINANCE: 執行訂單
    BINANCE-->>S1: 訂單結果
    S1-->>S4: 訂單狀態
    S4-->>S3: OrderResult
    
    Note over S6: 持倉管理
    S6->>S6: 監控持倉
    S6->>S4: POST /orders (移動停損)
    S4->>S1: 執行停損訂單
    S1->>BINANCE: 停損訂單
    BINANCE-->>S1: 執行結果
    S1-->>S4: 停損結果
    S4-->>S6: 停損狀態
```

## 資金劃轉流程圖

```mermaid
sequenceDiagram
    participant USER as 用戶/前端
    participant S12 as S12 Web UI
    participant S1 as S1 Exchange Connectors
    participant S6 as S6 Position Manager
    participant BINANCE as 幣安交易所
    participant REDIS as Redis
    
    Note over USER,BINANCE: 資金劃轉流程
    
    %% 人工劃轉
    USER->>S12: POST /treasury/transfer
    S12->>S12: 參數驗證
    S12->>REDIS: 獲取分散鎖
    S12->>S1: POST /xchg/treasury/transfer
    S1->>S1: 冪等檢查
    S1->>BINANCE: 資金劃轉 API
    BINANCE-->>S1: 劃轉結果
    S1-->>S12: TransferResponse
    S12->>REDIS: 釋放鎖
    S12-->>USER: 劃轉結果
    
    %% 自動劃轉
    Note over S6: 自動劃轉觸發
    S6->>S6: 檢查觸發條件
    S6->>S12: POST /treasury/transfer (內部)
    S12->>S12: 參數驗證
    S12->>REDIS: 獲取分散鎖
    S12->>S1: POST /xchg/treasury/transfer
    S1->>BINANCE: 資金劃轉 API
    BINANCE-->>S1: 劃轉結果
    S1-->>S12: TransferResponse
    S12-->>S6: 劃轉結果
    S6->>S6: 記錄劃轉日誌
```

## 配置管理流程圖

```mermaid
sequenceDiagram
    participant ADMIN as 管理員
    participant S12 as S12 Web UI
    participant S10 as S10 Config Service
    participant S3 as S3 Strategy Engine
    participant S6 as S6 Position Manager
    participant REDIS as Redis
    
    Note over ADMIN,REDIS: 配置管理流程
    
    ADMIN->>S12: 創建配置包
    S12->>S10: POST /bundles
    S10->>S10: 創建 DRAFT 配置
    
    ADMIN->>S12: 模擬分析
    S12->>S10: POST /simulate
    S10->>S10: 執行模擬
    S10-->>S12: 模擬結果
    
    ADMIN->>S12: 進場配置
    S12->>S10: POST /bundles/{id}/stage
    S10->>S10: 進入 STAGED
    
    ADMIN->>S12: 推廣配置
    S12->>S10: POST /promote
    S10->>REDIS: 廣播 cfg:events
    REDIS->>S3: 配置更新事件
    REDIS->>S6: 配置更新事件
    S3->>S10: GET /active
    S6->>S10: GET /active
    S10-->>S3: 新配置
    S10-->>S6: 新配置
```

## 對帳流程圖

```mermaid
sequenceDiagram
    participant SCHEDULER as 排程系統
    participant S12 as S12 Web UI
    participant S5 as S5 Reconciler
    participant S4 as S4 Order Router
    participant BINANCE as 幣安交易所
    participant DB as ArangoDB
    
    Note over SCHEDULER,DB: 對帳流程
    
    SCHEDULER->>S5: POST /reconcile
    S5->>BINANCE: 獲取交易所數據
    BINANCE-->>S5: 訂單/持倉數據
    S5->>DB: 獲取本地數據
    DB-->>S5: 本地訂單/持倉
    
    Note over S5: 數據對比分析
    
    alt 發現孤兒訂單 (API有/DB無)
        S5->>S4: POST /cancel
        S4->>BINANCE: 取消訂單
        BINANCE-->>S4: 取消結果
        S4-->>S5: 取消狀態
    else 發現孤兒訂單 (DB有/API無)
        S5->>DB: 清理本地訂單狀態
    else 發現孤兒持倉
        S5->>S4: POST /cancel (平倉)
        S4->>BINANCE: 平倉訂單
        BINANCE-->>S4: 平倉結果
        S4-->>S5: 平倉狀態
    end
    
    S5->>DB: 記錄對帳結果
    S5-->>SCHEDULER: 對帳完成
```

## 監控告警流程圖

```mermaid
sequenceDiagram
    participant S11 as S11 Metrics & Health
    participant S12 as S12 Web UI
    participant SERVICES as 各服務
    participant REDIS as Redis
    participant DB as ArangoDB
    
    Note over S11,DB: 監控告警流程
    
    %% 指標收集
    SERVICES->>REDIS: 推送指標數據
    REDIS->>S11: 指標事件
    S11->>S11: 聚合指標
    S11->>DB: 存儲指標
    
    %% 告警檢查
    S11->>S11: 檢查告警規則
    alt 觸發告警
        S11->>DB: 記錄告警
        S11->>S12: 推送告警
        S12->>S12: 顯示告警
    end
    
    %% 健康檢查
    S12->>SERVICES: GET /health
    SERVICES-->>S12: 健康狀態
    S12->>S11: 健康指標
    S11->>S11: 聚合健康狀態
    S11->>DB: 存儲健康數據
    
    %% 指標查詢
    S12->>S11: GET /metrics
    S11->>DB: 查詢指標
    DB-->>S11: 指標數據
    S11-->>S12: 指標回應
```
