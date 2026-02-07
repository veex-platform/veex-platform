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

		// Development and Production Allowlist
		allowedOrigins := []string{
			"http://localhost:5173", // Vite default
			"http://localhost:3000", // React default
			"http://localhost:80",   // Docker Nginx
			"http://studio.localhost",
			"http://admin.localhost",
			"https://studio.veexplatform.com",
			"https://admin.veexplatform.com",
		}

		allowed := false
		for _, o := range allowedOrigins {
			if o == origin {
				allowed = true
				break
			}
		}

		// Also allow if explicitly set via ENV
		isWildcard := false
		if envOrigin := os.Getenv("ALLOWED_ORIGINS"); envOrigin != "" {
			if envOrigin == "*" {
				allowed = true
				isWildcard = true
			} else if envOrigin == origin {
				allowed = true
			}
		}

		if allowed {
			if isWildcard {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		}

		if c.Request.Method == "OPTIONS" {
			// Even if origin is not in allowlist, we might want to return 204 for OPTIONS to avoid some browser console errors,
			// but strictly speaking, we should only return success if allowed.
			// Ideally, for unauthorized origins, we just let it fail or return 403.
			if allowed {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}
