package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DevicesHandler struct {
	db *sql.DB
}

func NewDevicesHandler(database *sql.DB) *DevicesHandler {
	return &DevicesHandler{db: database}
}

// ListDevices godoc
// @Summary List devices
// @Description Get paginated list of registered devices with optional fleet filter
// @Tags Devices
// @Accept json
// @Produce json
// @Param fleet_id query string false "Filter by fleet ID"
// @Param limit query int false "Maximum results" default(100)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/devices [get]
func (h *DevicesHandler) ListDevices(c *gin.Context) {
	fleetID := c.Query("fleet_id")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	var rows *sql.Rows
	var err error

	if fleetID != "" {
		rows, err = h.db.Query(`
			SELECT id, firmware_version, fleet_id, last_seen, metadata 
			FROM devices 
			WHERE fleet_id = ? 
			LIMIT ? OFFSET ?`, fleetID, limit, offset)
	} else {
		rows, err = h.db.Query(`
			SELECT id, firmware_version, fleet_id, last_seen, metadata 
			FROM devices 
			LIMIT ? OFFSET ?`, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var devices []gin.H
	for rows.Next() {
		var id, lastSeen string
		var firmwareVersion, fleetIDCol, metadata sql.NullString

		rows.Scan(&id, &firmwareVersion, &fleetIDCol, &lastSeen, &metadata)

		device := gin.H{
			"id":        id,
			"last_seen": lastSeen,
		}

		if firmwareVersion.Valid {
			device["firmware_version"] = firmwareVersion.String
		}
		if fleetIDCol.Valid {
			device["fleet_id"] = fleetIDCol.String
		}
		if metadata.Valid {
			device["metadata"] = metadata.String
		}

		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// GetDevice godoc
// @Summary Get device details
// @Description Get detailed information about a specific device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {string} string "Device not found"
// @Router /api/v1/admin/devices/{id} [get]
func (h *DevicesHandler) GetDevice(c *gin.Context) {
	deviceID := c.Param("id")

	var id, lastSeen string
	var firmwareVersion, fleetID, metadata sql.NullString

	err := h.db.QueryRow(`
		SELECT id, firmware_version, fleet_id, last_seen, metadata 
		FROM devices WHERE id = ?`, deviceID).
		Scan(&id, &firmwareVersion, &fleetID, &lastSeen, &metadata)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Device not found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	device := gin.H{
		"id":        id,
		"last_seen": lastSeen,
	}

	if firmwareVersion.Valid {
		device["firmware_version"] = firmwareVersion.String
	}
	if fleetID.Valid {
		device["fleet_id"] = fleetID.String
	}
	if metadata.Valid {
		device["metadata"] = metadata.String
	}

	c.JSON(http.StatusOK, device)
}

// UpdateDevice godoc
// @Summary Update device
// @Description Update device firmware version, fleet assignment, or metadata
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Param device body map[string]interface{} true "Device update payload"
// @Success 200 {object} map[string]string
// @Failure 404 {string} string "Device not found"
// @Router /api/v1/admin/devices/{id} [put]
func (h *DevicesHandler) UpdateDevice(c *gin.Context) {
	deviceID := c.Param("id")

	var update struct {
		FirmwareVersion string `json:"firmware_version"`
		FleetID         string `json:"fleet_id"`
		Metadata        string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE devices 
		SET firmware_version = ?, fleet_id = ?, metadata = ? 
		WHERE id = ?`, update.FirmwareVersion, update.FleetID, update.Metadata, deviceID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.String(http.StatusNotFound, "Device not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// DeleteDevice godoc
// @Summary Delete device
// @Description Permanently delete a device and all its telemetry data
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} map[string]string
// @Failure 404 {string} string "Device not found"
// @Router /api/v1/admin/devices/{id} [delete]
func (h *DevicesHandler) DeleteDevice(c *gin.Context) {
	deviceID := c.Param("id")

	// Delete telemetry first (foreign key)
	h.db.Exec(`DELETE FROM signals WHERE device_id = ?`, deviceID)

	result, err := h.db.Exec(`DELETE FROM devices WHERE id = ?`, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.String(http.StatusNotFound, "Device not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// GetDeviceTelemetry godoc
// @Summary Get device telemetry
// @Description Get recent telemetry signals from a specific device
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Param limit query int false "Maximum results" default(100)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/devices/{id}/telemetry [get]
func (h *DevicesHandler) GetDeviceTelemetry(c *gin.Context) {
	deviceID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	rows, err := h.db.Query(`
		SELECT signal_type, value, unit, timestamp 
		FROM signals 
		WHERE device_id = ? 
		ORDER BY timestamp DESC 
		LIMIT ?`, deviceID, limit)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var signals []gin.H
	for rows.Next() {
		var signalType, value, unit, timestamp string
		rows.Scan(&signalType, &value, &unit, &timestamp)

		signals = append(signals, gin.H{
			"type":      signalType,
			"value":     value,
			"unit":      unit,
			"timestamp": timestamp,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"device_id": deviceID,
		"signals":   signals,
		"count":     len(signals),
	})
}
