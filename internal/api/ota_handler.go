package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type OTAHandler struct {
	db *sql.DB
}

func NewOTAHandler(database *sql.DB) *OTAHandler {
	return &OTAHandler{db: database}
}

// ListDevices returns all registered devices
func (h *OTAHandler) ListDevices(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, registered_at, last_seen, firmware_version, fleet_id FROM devices ORDER BY last_seen DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var devices []any
	for rows.Next() {
		var id, version, fleet string
		var reg, seen time.Time
		rows.Scan(&id, &reg, &seen, &version, &fleet)
		devices = append(devices, gin.H{
			"id":               id,
			"registered_at":    reg,
			"last_seen":        seen,
			"firmware_version": version,
			"fleet_id":         fleet,
		})
	}
	c.JSON(http.StatusOK, devices)
}

// CreateFleet creates a new fleet
func (h *OTAHandler) CreateFleet(c *gin.Context) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := h.db.Exec("INSERT INTO fleets (id, name, description) VALUES (?, ?, ?)", req.ID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

// ListFleets returns all fleets
func (h *OTAHandler) ListFleets(c *gin.Context) {
	rows, err := h.db.Query("SELECT id, name, description, created_at FROM fleets")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var fleets []any
	for rows.Next() {
		var id, name, desc string
		var created time.Time
		rows.Scan(&id, &name, &desc, &created)
		fleets = append(fleets, gin.H{
			"id":          id,
			"name":        name,
			"description": desc,
			"created_at":  created,
		})
	}
	c.JSON(http.StatusOK, fleets)
}

// CreateCampaign starts a new OTA campaign
func (h *OTAHandler) CreateCampaign(c *gin.Context) {
	var req struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		ArtifactID    string `json:"artifact_id"`
		TargetFleetID string `json:"target_fleet_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Deactivate any currently active campaigns for the same fleet to ensure one source of truth
	h.db.Exec("UPDATE ota_campaigns SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE target_fleet_id = ? AND status = 'active'", req.TargetFleetID)

	_, err := h.db.Exec(`
		INSERT INTO ota_campaigns (id, name, artifact_id, target_fleet_id, status, started_at) 
		VALUES (?, ?, ?, ?, 'active', CURRENT_TIMESTAMP)`,
		req.ID, req.Name, req.ArtifactID, req.TargetFleetID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}
