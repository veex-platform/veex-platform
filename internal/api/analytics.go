package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	db *sql.DB
}

func NewAnalyticsHandler(database *sql.DB) *AnalyticsHandler {
	return &AnalyticsHandler{db: database}
}

// GetDashboard returns real-time dashboard metrics
// @Summary Get dashboard metrics
// @Description Get real-time metrics for the platform dashboard
// @Tags Analytics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/analytics/dashboard [get]
func (h *AnalyticsHandler) GetDashboard(c *gin.Context) {
	dashboard := gin.H{}

	// Total devices
	var totalDevices int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&totalDevices)
	dashboard["total_devices"] = totalDevices

	// Active devices (seen in last 24h)
	var activeDevices int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE last_seen > datetime('now', '-1 day')`).Scan(&activeDevices)
	dashboard["active_devices"] = activeDevices

	// Total fleets
	var totalFleets int
	h.db.QueryRow(`SELECT COUNT(*) FROM fleets`).Scan(&totalFleets)
	dashboard["total_fleets"] = totalFleets

	// Active campaigns
	var activeCampaigns int
	h.db.QueryRow(`SELECT COUNT(*) FROM ota_campaigns WHERE status = 'active'`).Scan(&activeCampaigns)
	dashboard["active_campaigns"] = activeCampaigns

	// Total telemetry signals (last 24h)
	var recentSignals int
	h.db.QueryRow(`SELECT COUNT(*) FROM signals WHERE timestamp > datetime('now', '-1 day')`).Scan(&recentSignals)
	dashboard["signals_24h"] = recentSignals

	// Recent events (last 100)
	rows, _ := h.db.Query(`
		SELECT event_type, resource_type, resource_id, timestamp 
		FROM events 
		ORDER BY timestamp DESC 
		LIMIT 100`)
	defer rows.Close()

	var events []gin.H
	for rows.Next() {
		var eventType, resourceType, resourceID, timestamp string
		rows.Scan(&eventType, &resourceType, &resourceID, &timestamp)

		events = append(events, gin.H{
			"type":          eventType,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"timestamp":     timestamp,
		})
	}
	dashboard["recent_events"] = events

	c.JSON(http.StatusOK, dashboard)
}

// GetDeviceSummary returns device statistics
// @Summary Get device summary statistics
// @Description Get aggregated device statistics by fleet and firmware version
// @Tags Analytics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/analytics/devices/summary [get]
func (h *AnalyticsHandler) GetDeviceSummary(c *gin.Context) {
	summary := gin.H{}

	// Total devices
	var total int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&total)
	summary["total"] = total

	// By fleet
	rows, _ := h.db.Query(`
		SELECT fleet_id, COUNT(*) as count 
		FROM devices 
		WHERE fleet_id IS NOT NULL 
		GROUP BY fleet_id`)
	defer rows.Close()

	fleetCounts := make(map[string]int)
	for rows.Next() {
		var fleetID string
		var count int
		rows.Scan(&fleetID, &count)
		fleetCounts[fleetID] = count
	}
	summary["by_fleet"] = fleetCounts

	// By firmware version
	rows2, _ := h.db.Query(`
		SELECT firmware_version, COUNT(*) as count 
		FROM devices 
		WHERE firmware_version IS NOT NULL 
		GROUP BY firmware_version`)
	defer rows2.Close()

	firmwareCounts := make(map[string]int)
	for rows2.Next() {
		var version string
		var count int
		rows2.Scan(&version, &count)
		firmwareCounts[version] = count
	}
	summary["by_firmware_version"] = firmwareCounts

	c.JSON(http.StatusOK, summary)
}

// GetOTAMetrics returns OTA campaign success metrics
// @Summary Get OTA success metrics
// @Description Get OTA campaign statistics including success rate
// @Tags Analytics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/analytics/ota/success-rate [get]
func (h *AnalyticsHandler) GetOTAMetrics(c *gin.Context) {
	metrics := gin.H{}

	// Total campaigns
	var totalCampaigns int
	h.db.QueryRow(`SELECT COUNT(*) FROM ota_campaigns`).Scan(&totalCampaigns)
	metrics["total_campaigns"] = totalCampaigns

	// Completed campaigns
	var completedCampaigns int
	h.db.QueryRow(`SELECT COUNT(*) FROM ota_campaigns WHERE status = 'completed'`).Scan(&completedCampaigns)
	metrics["completed_campaigns"] = completedCampaigns

	// Total success/failure counts
	var totalSuccess, totalFailure int
	h.db.QueryRow(`SELECT COALESCE(SUM(success_count), 0), COALESCE(SUM(failure_count), 0) FROM ota_campaigns`).
		Scan(&totalSuccess, &totalFailure)

	metrics["total_success"] = totalSuccess
	metrics["total_failure"] = totalFailure

	// Calculate success rate
	totalAttempts := totalSuccess + totalFailure
	if totalAttempts > 0 {
		metrics["success_rate"] = float64(totalSuccess) / float64(totalAttempts) * 100.0
	} else {
		metrics["success_rate"] = 0.0
	}

	c.JSON(http.StatusOK, metrics)
}
