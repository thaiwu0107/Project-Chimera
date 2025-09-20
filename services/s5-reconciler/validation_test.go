package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s5-reconciler/internal/apispec"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestReconcile_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServer()

	tests := []struct {
		name           string
		request        apispec.ReconcileRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid reconcile request",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  72,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid reconcile with ORDERS mode",
			request: apispec.ReconcileRequest{
				Mode:         "ORDERS",
				DryRun:       true,
				OrphanPolicy: "RECLAIM_IF_SAFE",
				TimeWindowH:  24,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid reconcile with POSITIONS mode",
			request: apispec.ReconcileRequest{
				Mode:         "POSITIONS",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  48,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid mode",
			request: apispec.ReconcileRequest{
				Mode:         "INVALID",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  72,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "time window too small",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  0, // 小於 1
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
		{
			name: "time window too large",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  200, // 超過 168
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
		{
			name: "invalid orphan policy",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "INVALID_POLICY",
				TimeWindowH:  72,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Business logic validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建測試路由
			r := gin.New()
			r.POST("/reconcile", server.Reconcile)

			// 準備請求
			reqBody := `{"mode":"ALL","dry_run":false,"orphan_policy":"CONSERVATIVE","time_window_h":72}`
			req := httptest.NewRequest("POST", "/reconcile", strings.NewReader(reqBody))
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

func TestValidateReconcileRequest_BusinessLogic(t *testing.T) {
	server := NewServer()

	tests := []struct {
		name    string
		request apispec.ReconcileRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  72,
			},
			wantErr: false,
		},
		{
			name: "time window too small",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  0,
			},
			wantErr: true,
			errMsg:  "time_window_h must be between 1 and 168 hours",
		},
		{
			name: "time window too large",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "CONSERVATIVE",
				TimeWindowH:  200,
			},
			wantErr: true,
			errMsg:  "time_window_h must be between 1 and 168 hours",
		},
		{
			name: "invalid orphan policy",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "INVALID_POLICY",
				TimeWindowH:  72,
			},
			wantErr: true,
			errMsg:  "orphan_policy must be either RECLAIM_IF_SAFE or CONSERVATIVE",
		},
		{
			name: "empty orphan policy (valid)",
			request: apispec.ReconcileRequest{
				Mode:         "ALL",
				DryRun:       false,
				OrphanPolicy: "", // 空字符串是有效的
				TimeWindowH:  72,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateReconcileRequest(&tt.request)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
