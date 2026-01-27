package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db        *sql.DB
	startTime time.Time
}

func NewHealthHandler(database *sql.DB) *HealthHandler {
	return &HealthHandler{
		db:        database,
		startTime: time.Now(),
	}
}

// Health checks platform health status
// @Summary Platform health check
// @Description Get current health status of the platform including database connectivity
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /api/v1/admin/health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	health := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
	}

	// Check database connectivity
	err := h.db.Ping()
	if err != nil {
		health["status"] = "unhealthy"
		health["database"] = "unreachable"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}
	health["database"] = "connected"

	// Get database stats
	var deviceCount, fleetCount, campaignCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM fleets`).Scan(&fleetCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM ota_campaigns`).Scan(&campaignCount)

	health["devices"] = deviceCount
	health["fleets"] = fleetCount
	health["campaigns"] = campaignCount

	c.JSON(http.StatusOK, health)
}

// Version returns API version information
// @Summary Get API version
// @Description Get version information about the VEEX Platform API
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/version [get]
func (h *HealthHandler) Version(c *gin.Context) {
	version := gin.H{
		"api_version": "v1",
		"platform":    "veex-platform",
		"version":     "1.0.0", // TODO: Read from build info
		"go_version":  runtime.Version(),
		"build_time":  "2026-01-24", // TODO: Read from build info
	}

	c.JSON(http.StatusOK, version)
}

// Metrics returns Prometheus-format metrics
// @Summary Prometheus metrics
// @Description Get platform metrics in Prometheus exposition format
// @Tags Health
// @Produce text/plain
// @Success 200 {string} string
// @Router /api/v1/metrics [get]
func (h *HealthHandler) Metrics(c *gin.Context) {
	var deviceCount, activeDeviceCount, fleetCount, campaignCount, signalCount int

	h.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&deviceCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE last_seen > datetime('now', '-1 hour')`).Scan(&activeDeviceCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM fleets`).Scan(&fleetCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM ota_campaigns WHERE status = 'active'`).Scan(&campaignCount)
	h.db.QueryRow(`SELECT COUNT(*) FROM signals WHERE timestamp > datetime('now', '-1 hour')`).Scan(&signalCount)

	writer := c.Writer
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(writer, "# HELP veex_devices_total Total number of registered devices\n")
	fmt.Fprintf(writer, "# TYPE veex_devices_total gauge\n")
	fmt.Fprintf(writer, "veex_devices_total %d\n\n", deviceCount)

	fmt.Fprintf(writer, "# HELP veex_devices_active Number of devices active in the last hour\n")
	fmt.Fprintf(writer, "# TYPE veex_devices_active gauge\n")
	fmt.Fprintf(writer, "veex_devices_active %d\n\n", activeDeviceCount)

	fmt.Fprintf(writer, "# HELP veex_fleets_total Total number of device fleets\n")
	fmt.Fprintf(writer, "# TYPE veex_fleets_total gauge\n")
	fmt.Fprintf(writer, "veex_fleets_total %d\n\n", fleetCount)

	fmt.Fprintf(writer, "# HELP veex_campaigns_active Number of active OTA campaigns\n")
	fmt.Fprintf(writer, "# TYPE veex_campaigns_active gauge\n")
	fmt.Fprintf(writer, "veex_campaigns_active %d\n\n", campaignCount)

	fmt.Fprintf(writer, "# HELP veex_signals_last_hour Number of telemetry signals in the last hour\n")
	fmt.Fprintf(writer, "# TYPE veex_signals_last_hour counter\n")
	fmt.Fprintf(writer, "veex_signals_last_hour %d\n\n", signalCount)

	fmt.Fprintf(writer, "# HELP veex_uptime_seconds Platform uptime in seconds\n")
	fmt.Fprintf(writer, "# TYPE veex_uptime_seconds gauge\n")
	fmt.Fprintf(writer, "veex_uptime_seconds %d\n\n", int(time.Since(h.startTime).Seconds()))
}
