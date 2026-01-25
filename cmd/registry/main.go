package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/veex-platform/veex-platform/docs"
	_ "github.com/veex-platform/veex-platform/docs" // Swagger docs
	"github.com/veex-platform/veex-platform/internal/api"
	"github.com/veex-platform/veex-platform/internal/db"
	"github.com/veex-platform/veex-platform/internal/observability"
	"github.com/veex-platform/veex-platform/internal/registry"
)

// @title VEEX Platform API
// @version 1.0
// @description Enterprise-grade Industrial IoT Platform API for device management, OTA updates, and telemetry.
// @termsOfService https://github.com/veex-platform

// @contact.name VEEX Platform
// @contact.url https://github.com/veex-platform
// @contact.email support@veex.dev

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @schemes http https
// @BasePath /

func main() {
	// Dynamically set Swagger host to empty to use the current domain
	docs.SwaggerInfo.Host = ""

	// Configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage"
	}

	// Ensure directories exist
	os.MkdirAll(storageDir, 0755)
	os.MkdirAll(dataDir, 0755)

	templatesDir := os.Getenv("TEMPLATES_DIR")
	if templatesDir == "" {
		templatesDir = "../veex-templates"
	}

	// Initialize Database
	database, err := db.InitDB(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Handlers
	rh := &registry.RegistryHandler{
		StorageDir: storageDir,
		Devices:    registry.NewDeviceRegistry(database),
	}
	oh := observability.NewObsHandler(database)

	// API Handlers
	devicesHandler := api.NewDevicesHandler(database)
	fleetsHandler := api.NewFleetsHandler(database)
	otaHandler := api.NewOTAHandler(database)
	analyticsHandler := api.NewAnalyticsHandler(database)
	healthHandler := api.NewHealthHandler(database)
	templatesHandler := api.NewTemplatesHandler(templatesDir)

	// Initialize Gin
	router := gin.Default()

	// CORS Middleware
	router.Use(func(c *gin.Context) {
		origins := os.Getenv("ALLOWED_ORIGINS")
		if origins == "" {
			origins = "*"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", origins)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Discovery Middleware: Detects the public Base URL for the platform
	router.Use(func(c *gin.Context) {
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			// Fallback to dynamic detection from headers
			scheme := "http"
			if c.Request.Header.Get("X-Forwarded-Proto") != "" {
				scheme = c.Request.Header.Get("X-Forwarded-Proto")
			} else if c.Request.TLS != nil {
				scheme = "https"
			}

			host := c.Request.Host
			if c.Request.Header.Get("X-Forwarded-Host") != "" {
				host = c.Request.Header.Get("X-Forwarded-Host")
			}
			baseURL = fmt.Sprintf("%s://%s", scheme, host)
		}

		// Store in context for handlers to use
		c.Set("BaseURL", baseURL)
		c.Next()
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Devices
		devices := v1.Group("/devices")
		{
			devices.GET("", devicesHandler.ListDevices)
			devices.GET("/:id", devicesHandler.GetDevice)
			devices.PUT("/:id", devicesHandler.UpdateDevice)
			devices.DELETE("/:id", devicesHandler.DeleteDevice)
			devices.GET("/:id/telemetry", devicesHandler.GetDeviceTelemetry)
		}

		// Fleets
		fleets := v1.Group("/fleets")
		{
			fleets.GET("", fleetsHandler.ListFleets)
			fleets.POST("", fleetsHandler.CreateFleet)
			fleets.GET("/:id", fleetsHandler.GetFleet)
			fleets.PUT("/:id", fleetsHandler.UpdateFleet)
			fleets.DELETE("/:id", fleetsHandler.DeleteFleet)
			fleets.POST("/:id/devices", fleetsHandler.AddDeviceToFleet)
		}

		// OTA Campaigns
		ota := v1.Group("/ota/campaigns")
		{
			ota.GET("", otaHandler.ListCampaigns)
			ota.POST("", otaHandler.CreateCampaign)
			ota.GET("/:id", otaHandler.GetCampaign)
			ota.POST("/:id/start", otaHandler.StartCampaign)
			ota.PUT("/:id/pause", otaHandler.PauseCampaign)
			ota.PUT("/:id/resume", otaHandler.ResumeCampaign)
		}

		// Analytics
		analytics := v1.Group("/analytics")
		{
			analytics.GET("/dashboard", analyticsHandler.GetDashboard)
			analytics.GET("/devices/summary", analyticsHandler.GetDeviceSummary)
			analytics.GET("/ota/success-rate", analyticsHandler.GetOTAMetrics)
		}

		// Health & Monitoring
		v1.GET("/health", healthHandler.Health)
		v1.GET("/version", healthHandler.Version)
		v1.GET("/metrics", healthHandler.Metrics)

		// Templates
		v1.GET("/templates", templatesHandler.ListTemplates)

		// OTA & Fleet Management
		v1.GET("/ota/devices", otaHandler.ListDevices)
		v1.GET("/ota/fleets", otaHandler.ListFleets)
		v1.POST("/ota/fleets", otaHandler.CreateFleet)
		v1.POST("/ota/campaigns", otaHandler.CreateCampaign)

		// Legacy Registry endpoints (kept for backwards compatibility)
		registry := v1.Group("/registry")
		{
			registry.POST("/upload", gin.WrapF(rh.Upload))
			registry.GET("/download", gin.WrapF(rh.Download))
			registry.GET("/check-update", gin.WrapF(rh.CheckUpdate))
			registry.POST("/register", gin.WrapF(rh.Devices.Register))
		}

		// Build endpoint
		v1.POST("/build", gin.WrapF(rh.Build))

	}

	// Observability (Root level for deployment compatibility)
	router.POST("/signals", gin.WrapF(oh.Ingest))
	router.GET("/dashboard", gin.WrapF(oh.Dashboard))

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Print startup banner
	fmt.Println("VEEX Platform API Server")
	fmt.Println("========================")
	fmt.Printf("Server: http://localhost:%s\n", port)
	fmt.Printf("API Documentation: http://localhost:%s/swagger/index.html\n", port)
	fmt.Printf("Health Check: http://localhost:%s/api/v1/health\n", port)
	fmt.Printf("Metrics: http://localhost:%s/api/v1/metrics\n", port)
	fmt.Printf("Listening on port %s...\n", port)

	// Start server
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
