package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s2-feature/dao"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecomputeFeatures_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS2_FEATUREServer()

	tests := []struct {
		name           string
		request        dao.RecomputeFeaturesRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid recompute request",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT", "ETHUSDT"},
				Windows: []string{"1h", "4h", "1d"},
				Force:   false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid recompute with force",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{"1h"},
				Force:   true,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "empty symbols",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{}, // 空數組
				Windows: []string{"1h"},
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid symbol format",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"btc-usdt"}, // 小寫和連字符
				Windows: []string{"1h"},
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "symbol too short",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTC"}, // 少於 3 個字符
				Windows: []string{"1h"},
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "empty windows",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{}, // 空數組
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid window",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{"2h"}, // 不在允許的枚舉中
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "multiple invalid windows",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{"1h", "2h", "3h"}, // 混合有效和無效
				Force:   false,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/features/recompute", server.RecomputeFeatures)

			// 準備請求
			reqBody := `{"symbols":["BTCUSDT"],"windows":["1h"],"force":false}`
			req := httptest.NewRequest("POST", "/features/recompute", strings.NewReader(reqBody))
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

func TestRecomputeFeatures_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS2_FEATUREServer()

	tests := []struct {
		name           string
		request        dao.RecomputeFeaturesRequest
		expectedStatus int
		description    string
	}{
		{
			name: "single symbol single window",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{"1h"},
				Force:   false,
			},
			expectedStatus: http.StatusOK,
			description:    "最小有效請求",
		},
		{
			name: "multiple symbols multiple windows",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT", "ETHUSDT", "ADAUSDT"},
				Windows: []string{"1m", "5m", "1h", "4h", "1d"},
				Force:   true,
			},
			expectedStatus: http.StatusOK,
			description:    "最大有效請求",
		},
		{
			name: "all valid windows",
			request: dao.RecomputeFeaturesRequest{
				Symbols: []string{"BTCUSDT"},
				Windows: []string{"1m", "5m", "1h", "4h", "1d"},
				Force:   false,
			},
			expectedStatus: http.StatusOK,
			description:    "所有有效時間窗口",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/features/recompute", server.RecomputeFeatures)

			// 準備請求
			reqBody := `{"symbols":["BTCUSDT"],"windows":["1h"],"force":false}`
			req := httptest.NewRequest("POST", "/features/recompute", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// 執行請求
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// 驗證響應
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}
