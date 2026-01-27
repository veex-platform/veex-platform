package api

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		origin            string
		allowedEnv        string
		expectedOrigin    string
		expectCredentials bool
	}{
		{
			name:              "Default with Origin",
			origin:            "http://localhost:5173",
			allowedEnv:        "",
			expectedOrigin:    "http://localhost:5173",
			expectCredentials: true,
		},
		{
			name:              "Wildcard Env",
			origin:            "http://localhost:5173",
			allowedEnv:        "*",
			expectedOrigin:    "*",
			expectCredentials: false,
		},
		{
			name:              "Explicit Env",
			origin:            "http://other.com",
			allowedEnv:        "http://trusted.com",
			expectedOrigin:    "http://trusted.com",
			expectCredentials: true,
		},
		{
			name:              "No Origin No Env",
			origin:            "",
			allowedEnv:        "",
			expectedOrigin:    "*",
			expectCredentials: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ALLOWED_ORIGINS", tt.allowedEnv)
			defer os.Unsetenv("ALLOWED_ORIGINS")

			router := gin.New()
			router.Use(CORSMiddleware())
			router.GET("/test", func(c *gin.Context) {
				c.Status(200)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin = %v, want %v", got, tt.expectedOrigin)
			}

			cred := w.Header().Get("Access-Control-Allow-Credentials")
			if tt.expectCredentials {
				if cred != "true" {
					t.Errorf("expected Access-Control-Allow-Credentials: true")
				}
			} else {
				if cred != "" {
					t.Errorf("expected no Access-Control-Allow-Credentials, got %v", cred)
				}
			}
		})
	}
}
