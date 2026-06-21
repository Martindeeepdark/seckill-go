package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAdminTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireAdmin())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name           string
		roleHeader     string
		setHeader      bool
		wantStatus     int
		wantBodyCode   string
	}{
		{
			name:         "missing_header",
			setHeader:    false,
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "not_admin",
		},
		{
			name:         "empty_header",
			setHeader:    true,
			roleHeader:   "",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "not_admin",
		},
		{
			name:         "whitespace_only",
			setHeader:    true,
			roleHeader:   "   ",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "not_admin",
		},
		{
			name:         "non_admin_role",
			setHeader:    true,
			roleHeader:   "user",
			wantStatus:   http.StatusForbidden,
			wantBodyCode: "forbidden",
		},
		{
			name:       "admin_lower",
			setHeader:  true,
			roleHeader: "admin",
			wantStatus: http.StatusOK,
		},
		{
			name:       "admin_upper",
			setHeader:  true,
			roleHeader: "ADMIN",
			wantStatus: http.StatusOK,
		},
		{
			name:       "multi_role_with_admin",
			setHeader:  true,
			roleHeader: "operator,admin",
			wantStatus: http.StatusOK,
		},
		{
			name:         "multi_role_without_admin",
			setHeader:    true,
			roleHeader:   "operator,user",
			wantStatus:   http.StatusForbidden,
			wantBodyCode: "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupAdminTestRouter()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.setHeader {
				req.Header.Set(adminRoleHeader, tt.roleHeader)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantBodyCode != "" {
				var resp adminErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
				}
				if resp.Code != tt.wantBodyCode {
					t.Errorf("body code = %q, want %q", resp.Code, tt.wantBodyCode)
				}
				if resp.Success != false {
					t.Errorf("body success = %v, want false", resp.Success)
				}
			}
		})
	}
}

func TestIsAdminRole(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"admin", true},
		{"ADMIN", true},
		{"Admin", true},
		{"operator,admin", true},
		{"admin,superadmin", true},
		{"operator, admin", true},
		{"user", false},
		{"operator,user", false},
		{"superadmin", false},
		{"", false},
		{"   ", false},
		{"administrator", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isAdminRole(tt.input); got != tt.want {
				t.Errorf("isAdminRole(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
