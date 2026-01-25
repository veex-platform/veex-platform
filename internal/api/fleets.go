package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type FleetsHandler struct {
	db *sql.DB
}

func NewFleetsHandler(database *sql.DB) *FleetsHandler {
	return &FleetsHandler{db: database}
}

// ListFleets godoc
// @Summary List all fleets
// @Description Get list of all device fleets/groups with device counts
// @Tags Fleets
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/fleets [get]
func (h *FleetsHandler) ListFleets(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, name, description, created_at, updated_at 
		FROM fleets 
		ORDER BY created_at DESC`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var fleets []gin.H
	for rows.Next() {
		var id, name, created, updated string
		var description sql.NullString

		rows.Scan(&id, &name, &description, &created, &updated)

		fleet := gin.H{
			"id":         id,
			"name":       name,
			"created_at": created,
			"updated_at": updated,
		}

		if description.Valid {
			fleet["description"] = description.String
		}

		// Count devices in fleet
		var deviceCount int
		h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE fleet_id = ?`, id).Scan(&deviceCount)
		fleet["device_count"] = deviceCount

		fleets = append(fleets, fleet)
	}

	c.JSON(http.StatusOK, gin.H{
		"fleets": fleets,
		"total":  len(fleets),
	})
}

// CreateFleet godoc
// @Summary Create a new fleet
// @Description Create a new device fleet/group
// @Tags Fleets
// @Accept json
// @Produce json
// @Param fleet body map[string]interface{} true "Fleet creation payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {string} string "Invalid request"
// @Failure 409 {string} string "Fleet ID already exists"
// @Router /api/v1/fleets [post]
func (h *FleetsHandler) CreateFleet(c *gin.Context) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ID == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID and name are required"})
		return
	}

	_, err := h.db.Exec(`
		INSERT INTO fleets (id, name, description) 
		VALUES (?, ?, ?)`, req.ID, req.Name, req.Description)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.String(http.StatusConflict, "Fleet ID already exists")
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Log event
	h.logEvent("fleet_created", "fleet", req.ID, fmt.Sprintf(`{"name":"%s"}`, req.Name))

	c.JSON(http.StatusCreated, gin.H{
		"id":   req.ID,
		"name": req.Name,
	})
}

// GetFleet godoc
// @Summary Get fleet details
// @Description Get detailed information about a specific fleet
// @Tags Fleets
// @Produce json
// @Param id path string true "Fleet ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {string} string "Fleet not found"
// @Router /api/v1/fleets/{id} [get]
func (h *FleetsHandler) GetFleet(c *gin.Context) {
	fleetID := c.Param("id")

	var id, name, created, updated string
	var description sql.NullString

	err := h.db.QueryRow(`
		SELECT id, name, description, created_at, updated_at 
		FROM fleets WHERE id = ?`, fleetID).
		Scan(&id, &name, &description, &created, &updated)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Fleet not found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	fleet := gin.H{
		"id":         id,
		"name":       name,
		"created_at": created,
		"updated_at": updated,
	}

	if description.Valid {
		fleet["description"] = description.String
	}

	// Get device count
	var deviceCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE fleet_id = ?`, id).Scan(&deviceCount)
	fleet["device_count"] = deviceCount

	c.JSON(http.StatusOK, fleet)
}

// UpdateFleet godoc
// @Summary Update fleet
// @Description Update fleet name, description, or other metadata
// @Tags Fleets
// @Accept json
// @Produce json
// @Param id path string true "Fleet ID"
// @Param fleet body map[string]interface{} true "Fleet update payload"
// @Success 200 {object} map[string]string
// @Failure 404 {string} string "Fleet not found"
// @Router /api/v1/fleets/{id} [put]
func (h *FleetsHandler) UpdateFleet(c *gin.Context) {
	fleetID := c.Param("id")

	var update struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE fleets 
		SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`, update.Name, update.Description, fleetID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.String(http.StatusNotFound, "Fleet not found")
		return
	}

	h.logEvent("fleet_updated", "fleet", fleetID, fmt.Sprintf(`{"name":"%s"}`, update.Name))

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// DeleteFleet godoc
// @Summary Delete fleet
// @Description Permanently delete a fleet and remove device associations
// @Tags Fleets
// @Produce json
// @Param id path string true "Fleet ID"
// @Success 200 {object} map[string]string
// @Failure 404 {string} string "Fleet not found"
// @Router /api/v1/fleets/{id} [delete]
func (h *FleetsHandler) DeleteFleet(c *gin.Context) {
	fleetID := c.Param("id")

	// Remove fleet association from devices
	h.db.Exec(`UPDATE devices SET fleet_id = NULL WHERE fleet_id = ?`, fleetID)

	result, err := h.db.Exec(`DELETE FROM fleets WHERE id = ?`, fleetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.String(http.StatusNotFound, "Fleet not found")
		return
	}

	h.logEvent("fleet_deleted", "fleet", fleetID, "{}")

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// AddDeviceToFleet godoc
// @Summary Add device to fleet
// @Description Assign a device to a specific fleet
// @Tags Fleets
// @Accept json
// @Produce json
// @Param id path string true "Fleet ID"
// @Param device body map[string]interface{} true "Device assignment payload"
// @Success 200 {object} map[string]string
// @Failure 404 {string} string "Device not found"
// @Router /api/v1/fleets/{id}/devices [post]
func (h *FleetsHandler) AddDeviceToFleet(c *gin.Context) {
	fleetID := c.Param("id")

	var req struct {
		DeviceID string `json:"device_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.db.Exec(`UPDATE devices SET fleet_id = ? WHERE id = ?`, fleetID, req.DeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.String(http.StatusNotFound, "Device not found")
		return
	}

	h.logEvent("device_added_to_fleet", "fleet", fleetID, fmt.Sprintf(`{"device_id":"%s"}`, req.DeviceID))

	c.JSON(http.StatusOK, gin.H{"status": "added"})
}

func (h *FleetsHandler) logEvent(eventType, resourceType, resourceID, payload string) {
	h.db.Exec(`INSERT INTO events (event_type, resource_type, resource_id, payload) VALUES (?, ?, ?, ?)`,
		eventType, resourceType, resourceID, payload)
}
