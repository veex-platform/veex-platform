package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing configuration
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")

		allowedOrigin := "*"
		allowCredentials := false

		if allowedOriginsEnv != "" {
			// If explicitly set in environment
			if allowedOriginsEnv == "*" {
				allowedOrigin = "*"
				allowCredentials = false // Credentials cannot be used with wildcard origin
			} else {
				// For now, support a single explicit origin or simple check
				// In a more complex setup, we could split by comma and check
				allowedOrigin = allowedOriginsEnv
				allowCredentials = true
			}
		} else if origin != "" {
			// Dynamic origin echoing (Safe if combined with authentication)
			// This allows multiple frontends (Studio, Admin, etc.) to work
			allowedOrigin = origin
			allowCredentials = true
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)

		if allowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
