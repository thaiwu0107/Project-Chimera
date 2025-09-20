package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s10-config/dao"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBundleUpsert_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS10_CONFIGServer()

	tests := []struct {
		name           string
		request        dao.BundleUpsertRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid bundle upsert",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1", "factor2"},
				Rules:       []string{"rule1", "rule2"},
				Instruments: []string{"BTCUSDT", "ETHUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing bundle_id",
			request: dao.BundleUpsertRequest{
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "empty bundle_id",
			request: dao.BundleUpsertRequest{
				BundleID:    "", // 空字符串
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "bundle_id too long",
			request: dao.BundleUpsertRequest{
				BundleID:    strings.Repeat("A", 129), // 超過 128 字符
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid rev",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         0, // 必須大於 0
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "empty factors",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{}, // 空數組
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "empty rules",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{}, // 空數組
				Instruments: []string{"BTCUSDT"},
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "empty instruments",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{}, // 空數組
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid instrument format",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"btc-usdt"}, // 小寫和連字符
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "instrument too short",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTC"}, // 少於 3 個字符
				Status:      "DRAFT",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid status",
			request: dao.BundleUpsertRequest{
				BundleID:    "B-2025-01-01-001",
				Rev:         1,
				Factors:     []string{"factor1"},
				Rules:       []string{"rule1"},
				Instruments: []string{"BTCUSDT"},
				Status:      "INVALID", // 不在枚舉中
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/bundles", server.BundleUpsert)

			// 準備請求
			reqBody := `{"bundle_id":"B-2025-01-01-001","rev":1,"factors":["factor1"],"rules":["rule1"],"instruments":["BTCUSDT"],"status":"DRAFT"}`
			req := httptest.NewRequest("POST", "/bundles", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// 執行請求
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// 驗證響應
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestPromote_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS10_CONFIGServer()

	tests := []struct {
		name           string
		request        dao.PromoteRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid promote request",
			request: dao.PromoteRequest{
				BundleID:   "B-2025-01-01-001",
				ToRev:      2,
				Mode:       "CANARY",
				TrafficPct: 10,
				DurationH:  24,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid RAMP promote",
			request: dao.PromoteRequest{
				BundleID: "B-2025-01-01-001",
				ToRev:    2,
				Mode:     "RAMP",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid FULL promote",
			request: dao.PromoteRequest{
				BundleID: "B-2025-01-01-001",
				ToRev:    2,
				Mode:     "FULL",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid ROLLBACK promote",
			request: dao.PromoteRequest{
				BundleID: "B-2025-01-01-001",
				ToRev:    1,
				Mode:     "ROLLBACK",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing bundle_id",
			request: dao.PromoteRequest{
				ToRev: 2,
				Mode:  "CANARY",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid rev",
			request: dao.PromoteRequest{
				BundleID: "B-2025-01-01-001",
				ToRev:    0, // 必須大於 0
				Mode:     "CANARY",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid mode",
			request: dao.PromoteRequest{
				BundleID: "B-2025-01-01-001",
				ToRev:    2,
				Mode:     "INVALID",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "CANARY with invalid traffic_pct",
			request: dao.PromoteRequest{
				BundleID:   "B-2025-01-01-001",
				ToRev:      2,
				Mode:       "CANARY",
				TrafficPct: 60, // 超過 50
				DurationH:  24,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "CANARY with invalid duration_h",
			request: dao.PromoteRequest{
				BundleID:   "B-2025-01-01-001",
				ToRev:      2,
				Mode:       "CANARY",
				TrafficPct: 10,
				DurationH:  10, // 少於 24
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/promote", server.Promote)

			// 準備請求
			reqBody := `{"bundle_id":"B-2025-01-01-001","to_rev":2,"mode":"CANARY","traffic_pct":10,"duration_h":24}`
			req := httptest.NewRequest("POST", "/promote", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// 執行請求
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// 驗證響應
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}
