package main

import (
	"fmt"
	"log"
	"net/http"
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
	"github.com/veex-platform/veex-platform/internal/ws"
)

// @title VEEX Platform API
// @version 1.0
// @description Enterprise-grade Industrial IoT Platform API for device management, OTA updates, and telemetry.
// @termsOfService https://github.com/veex-platform

// @contact.name VEEX Platform
// @contact.url https://github.com/veex-platform
// @contact.email support@veexplatform.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @schemes https http
// @BasePath /

func main() {
	// Dynamically set Swagger host to empty to use the current domain
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{"https", "http"}

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
		templatesDir = "./veex-templates"
	}

	// Initialize Database
	database, err := db.InitDB(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Initialize Handlers
	rh := &registry.RegistryHandler{
		StorageDir: storageDir,
		Devices:    registry.NewDeviceRegistry(database, hub),
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
	router.Use(api.CORSMiddleware())

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
		// WebSocket Endpoint (Moved to /api/v1/ws to match Ingress routing)
		// @Summary Connect to Real-time WebSocket
		// @Description Establishes a WebSocket connection for real-time device events
		// @Tags System
		// @Success 101 {string} string "Switching Protocols"
		// @Router /api/v1/ws [get]
		v1.GET("/ws", func(c *gin.Context) {
			ws.ServeWs(hub, c)
		})

		// --- ADMIN GROUP ---
		admin := v1.Group("/admin")
		{
			admin.GET("/stats", func(c *gin.Context) {
				var devCount, fleetCount, campaignCount int
				database.QueryRow("SELECT COUNT(*) FROM devices").Scan(&devCount)
				database.QueryRow("SELECT COUNT(*) FROM fleets").Scan(&fleetCount)
				database.QueryRow("SELECT COUNT(*) FROM ota_campaigns").Scan(&campaignCount)

				c.JSON(http.StatusOK, gin.H{
					"devices":   devCount,
					"fleets":    fleetCount,
					"campaigns": campaignCount,
					"version":   "v1.1.1",
					"status":    "operational",
				})
			})
			admin.GET("/health", healthHandler.Health)

			// Resource Management
			devices := admin.Group("/devices")
			{
				devices.GET("", devicesHandler.ListDevices)
				devices.GET("/:id", devicesHandler.GetDevice)
				devices.PUT("/:id", devicesHandler.UpdateDevice)
				devices.DELETE("/:id", devicesHandler.DeleteDevice)
				devices.GET("/:id/telemetry", devicesHandler.GetDeviceTelemetry)
			}

			fleets := admin.Group("/fleets")
			{
				fleets.GET("", fleetsHandler.ListFleets)
				fleets.POST("", fleetsHandler.CreateFleet)
				fleets.GET("/:id", fleetsHandler.GetFleet)
				fleets.PUT("/:id", fleetsHandler.UpdateFleet)
				fleets.DELETE("/:id", fleetsHandler.DeleteFleet)
				fleets.POST("/:id/devices", fleetsHandler.AddDeviceToFleet)
			}

			ota := admin.Group("/ota")
			{
				ota.GET("/campaigns", otaHandler.ListCampaigns)
				ota.POST("/campaigns", otaHandler.CreateCampaign)
				ota.GET("/campaigns/:id", otaHandler.GetCampaign)
				ota.POST("/campaigns/:id/start", otaHandler.StartCampaign)
				ota.PUT("/campaigns/:id/pause", otaHandler.PauseCampaign)
				ota.PUT("/campaigns/:id/resume", otaHandler.ResumeCampaign)
			}

			admin.GET("/analytics/dashboard", analyticsHandler.GetDashboard)
		}

		// --- DEVELOPER GROUP (CLI & Studio) ---
		dev := v1.Group("/dev")
		{
			dev.POST("/build", gin.WrapF(rh.Build))
			dev.POST("/upload", gin.WrapF(rh.Upload))
			dev.GET("/templates", templatesHandler.ListTemplates)
			dev.POST("/deploy", otaHandler.InstantDeploy)
		}

		// --- RETAIN FOR COMPATIBILITY ---
		v1.GET("/health", healthHandler.Health)

		// --- RUNTIME GROUP (Devices) ---
		runtime := v1.Group("/runtime")
		{
			runtime.POST("/register", gin.WrapF(rh.Devices.Register))
			runtime.GET("/check-update", gin.WrapF(rh.CheckUpdate))
			runtime.GET("/download", gin.WrapF(rh.Download))
		}

		// --- COMPATIBILITY ALIASES (Legacy) ---
		v1.GET("/devices", devicesHandler.ListDevices)
		v1.GET("/fleets", fleetsHandler.ListFleets)
		v1.GET("/ota/campaigns", otaHandler.ListCampaigns)
		v1.POST("/ota/campaigns", otaHandler.CreateCampaign)
		v1.GET("/templates", templatesHandler.ListTemplates)
		v1.POST("/build", gin.WrapF(rh.Build))

		registry := v1.Group("/registry")
		{
			registry.POST("/upload", gin.WrapF(rh.Upload))
			registry.GET("/download", gin.WrapF(rh.Download))
			registry.GET("/check-update", gin.WrapF(rh.CheckUpdate))
			registry.POST("/register", gin.WrapF(rh.Devices.Register))
		}
	}

	// Observability & Native Routes
	router.POST("/signals", gin.WrapF(oh.Ingest))
	router.GET("/dashboard", gin.WrapF(oh.Dashboard))
	router.GET("/health", healthHandler.Health) // Root health

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Print startup banner
	fmt.Println("VEEX Platform API Server")
	fmt.Println("========================")
	fmt.Printf("Server: http://localhost:%s\n", port)
	fmt.Printf("API Documentation: http://localhost:%s/swagger/index.html\n", port)
	fmt.Printf("Health Check: http://localhost:%s/api/v1/health\n", port)
	fmt.Printf("Metrics: http://localhost:%s/api/v1/metrics\n", port)
	fmt.Printf("WebSocket: ws://localhost:%s/ws\n", port)
	fmt.Printf("Listening on port %s...\n", port)

	// Start server
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
