package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s4-router/dao"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateOrder_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS4_ROUTERServer()

	tests := []struct {
		name           string
		request        dao.OrderCmdRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid FUT order",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     20,
					ExecPolicy: dao.ExecPolicy{
						PreferMaker: true,
						MakerWaitMs: 3000,
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing intent_id",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					Symbol:       "BTCUSDT",
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     20,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid symbol format",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "btc-usdt", // 小寫和連字符
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     20,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "invalid market",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "OPTIONS", // 無效市場
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     20,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "FUT without leverage",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					// Leverage: 0, // 缺少槓桿
				},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
		{
			name: "invalid leverage range",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     200, // 超過 125
				},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
		{
			name: "OCO with invalid price relationship",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "SPOT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					ExecPolicy: dao.ExecPolicy{
						OCO: &dao.OCO{
							TakeProfitPx: 50000.0, // TP 價格
							StopLossPx:   51000.0, // SL 價格（錯誤：BUY 時 SL 應該低於 TP）
						},
					},
				},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
		{
			name: "TWAP with invalid slices",
			request: dao.OrderCmdRequest{
				Intent: dao.OrderIntent{
					IntentID:     "test-123",
					Symbol:       "BTCUSDT",
					Market:       "FUT",
					Kind:         "ENTRY",
					Side:         "BUY",
					NotionalUSDT: 100.0,
					Leverage:     20,
					ExecPolicy: dao.ExecPolicy{
						TWAPSlices: 15, // 超過 10
					},
				},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/orders", server.CreateOrder)

			// 準備請求
			reqBody := `{"intent":{"intent_id":"test-123","symbol":"BTCUSDT","market":"FUT","kind":"ENTRY","side":"BUY","notional_usdt":100.0,"leverage":20}}`
			req := httptest.NewRequest("POST", "/orders", strings.NewReader(reqBody))
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

func TestCancelOrder_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewS4_ROUTERServer()

	tests := []struct {
		name           string
		request        dao.CancelRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid cancel with order_id",
			request: dao.CancelRequest{
				OrderID: "123456789",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid cancel with client_order_id",
			request: dao.CancelRequest{
				ClientID: "client-123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "missing both order_id and client_order_id",
			request: dao.CancelRequest{
				// 兩個都為空
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Either order_id or client_order_id must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/cancel", server.CancelOrder)

			// 準備請求
			reqBody := `{"order_id":"123456789"}`
			req := httptest.NewRequest("POST", "/cancel", strings.NewReader(reqBody))
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

func TestValidateOrderIntent_BusinessLogic(t *testing.T) {
	server := NewS4_ROUTERServer()

	tests := []struct {
		name    string
		intent  dao.OrderIntent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid FUT order",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "FUT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				Leverage:     20,
			},
			wantErr: false,
		},
		{
			name: "FUT without leverage",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "FUT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				Leverage:     0,
			},
			wantErr: true,
			errMsg:  "leverage is required for FUT market",
		},
		{
			name: "FUT with invalid leverage",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "FUT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				Leverage:     200,
			},
			wantErr: true,
			errMsg:  "leverage must be between 1 and 125",
		},
		{
			name: "OCO with invalid BUY price relationship",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "SPOT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				ExecPolicy: dao.ExecPolicy{
					OCO: &dao.OCO{
						TakeProfitPx: 50000.0,
						StopLossPx:   51000.0, // 錯誤：BUY 時 SL 應該低於 TP
					},
				},
			},
			wantErr: true,
			errMsg:  "for BUY orders, take_profit_px must be greater than stop_loss_px",
		},
		{
			name: "OCO with invalid SELL price relationship",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "SPOT",
				Kind:         "ENTRY",
				Side:         "SELL",
				NotionalUSDT: 100.0,
				ExecPolicy: dao.ExecPolicy{
					OCO: &dao.OCO{
						TakeProfitPx: 51000.0, // 錯誤：SELL 時 TP 應該低於 SL
						StopLossPx:   50000.0,
					},
				},
			},
			wantErr: true,
			errMsg:  "for SELL orders, take_profit_px must be less than stop_loss_px",
		},
		{
			name: "TWAP with invalid slices",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "FUT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				Leverage:     20,
				ExecPolicy: dao.ExecPolicy{
					TWAPSlices: 15,
				},
			},
			wantErr: true,
			errMsg:  "TWAP slices must be between 1 and 10",
		},
		{
			name: "invalid maker wait time",
			intent: dao.OrderIntent{
				IntentID:     "test-123",
				Symbol:       "BTCUSDT",
				Market:       "FUT",
				Kind:         "ENTRY",
				Side:         "BUY",
				NotionalUSDT: 100.0,
				Leverage:     20,
				ExecPolicy: dao.ExecPolicy{
					MakerWaitMs: 15000, // 超過 10000
				},
			},
			wantErr: true,
			errMsg:  "maker_wait_ms must be between 0 and 10000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateOrderIntent(&tt.intent)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
