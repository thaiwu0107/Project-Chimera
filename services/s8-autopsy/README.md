# S8 Autopsy Generator

## 概述

S8 Autopsy Generator 是 Project Chimera 交易系統的交易復盤生成器，負責分析交易表現，生成詳細的復盤報告，幫助改進交易策略。

## 功能

- **交易分析**：分析交易表現和結果
- **復盤報告**：生成詳細的復盤報告
- **圖表生成**：創建交易圖表和視覺化
- **反事實分析**：分析替代策略的表現
- **同業比較**：與其他策略進行比較

## API 接口

### 健康檢查

- `GET /health` - 服務健康狀態檢查
- `GET /ready` - 服務就緒狀態檢查

### 復盤管理

- `POST /autopsy/{trade_id}` - 生成復盤報告

#### Generate Autopsy

**請求**：
```json
{
  "trade_id": "trade_001",
  "analysis_type": "FULL",
  "include_charts": true,
  "include_counterfactual": true,
  "peer_comparison": true
}
```

**回應**：
```json
{
  "report_id": "report_001",
  "trade_id": "trade_001",
  "status": "COMPLETED",
  "url": "https://minio.example.com/reports/report_001.pdf",
  "summary": {
    "pnl": 1250.75,
    "pnl_pct": 0.125,
    "max_drawdown": 0.05,
    "sharpe_ratio": 1.85,
    "win_rate": 0.68
  },
  "charts": [
    {
      "type": "PRICE_CHART",
      "url": "https://minio.example.com/charts/price_chart_001.png"
    },
    {
      "type": "PNL_CHART",
      "url": "https://minio.example.com/charts/pnl_chart_001.png"
    }
  ]
}
```

## 服務間交互

### 入向（被呼叫）
- **S12 Web UI** → `POST /autopsy/{trade_id}` - 手動生成復盤
- **排程系統** → `POST /autopsy/{trade_id}` - 定期復盤分析

### 出向（主動呼叫）
- **MinIO** → 存儲報告和圖表文件
- **數據庫** → 更新 autopsy_reports 集合
- **S12 Web UI** → 回傳報告 URL

## 復盤分析類型

### FULL 分析
- 完整的交易分析
- 包含所有圖表和指標
- 反事實分析
- 同業比較

### QUICK 分析
- 快速分析
- 基本指標和圖表
- 適合批量處理

### CUSTOM 分析
- 自定義分析類型
- 可選擇特定分析模組
- 靈活的配置選項

## 分析模組

### 價格分析
- 進場和出場時機
- 價格走勢分析
- 支撐阻力位分析

### 風險分析
- 最大回撤分析
- 風險調整收益
- 波動率分析

### 反事實分析
- 替代策略表現
- 不同參數影響
- 敏感性分析

### 同業比較
- 與其他策略比較
- 市場表現比較
- 基準比較

## 報告生成

### 報告格式
- **PDF**：完整報告
- **HTML**：互動式報告
- **JSON**：結構化數據

### 圖表類型
- 價格圖表
- PnL 圖表
- 風險指標圖表
- 反事實分析圖表

### 存儲策略
- MinIO 對象存儲
- 文件命名規範
- 訪問權限控制

## 失敗處理

### 重試機制
- 分析失敗自動重試
- 指數退避策略
- 最大重試次數

### 錯誤處理
- 數據缺失處理
- 分析異常處理
- 報告生成失敗處理

## 配置

服務使用以下配置：
- Redis：用於任務佇列
- ArangoDB：用於復盤數據存儲
- MinIO：用於文件存儲
- 端口：8088（可通過環境變量 PORT 覆蓋）

## 部署

```bash
# 構建
go build -o s8-autopsy .

# 運行
./s8-autopsy
```

## 監控

服務提供以下監控指標：
- 復盤生成延遲
- 報告生成成功率
- 圖表生成成功率
- 文件存儲成功率
- 分析準確性評分
