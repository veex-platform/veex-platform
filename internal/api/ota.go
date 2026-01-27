package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type OTAHandler struct {
	db *sql.DB
}

func NewOTAHandler(database *sql.DB) *OTAHandler {
	return &OTAHandler{db: database}
}

// ListCampaigns godoc
// @Summary List OTA campaigns
// @Description Get list of all OTA update campaigns with optional status filter
// @Tags OTA
// @Produce json
// @Param status query string false "Filter by status (draft, active, paused, completed)"
// @Param limit query int false "Maximum results" default(100)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/ota/campaigns [get]
func (h *OTAHandler) ListCampaigns(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = h.db.Query(`
			SELECT id, name, artifact_id, target_fleet_id, target_device_id, status, created_at, started_at, completed_at, success_count, failure_count 
			FROM ota_campaigns 
			WHERE status = ? 
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, status, limit, offset)
	} else {
		rows, err = h.db.Query(`
			SELECT id, name, artifact_id, target_fleet_id, target_device_id, status, created_at, started_at, completed_at, success_count, failure_count 
			FROM ota_campaigns 
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var campaigns []gin.H
	for rows.Next() {
		var id, name, artifactID, status, created string
		var fleetID, deviceID, started, completed sql.NullString
		var successCount, failureCount int

		rows.Scan(&id, &name, &artifactID, &fleetID, &deviceID, &status, &created, &started, &completed, &successCount, &failureCount)

		campaign := gin.H{
			"id":            id,
			"name":          name,
			"artifact_id":   artifactID,
			"status":        status,
			"created_at":    created,
			"success_count": successCount,
			"failure_count": failureCount,
		}

		if fleetID.Valid {
			campaign["target_fleet_id"] = fleetID.String
		}
		if deviceID.Valid {
			campaign["target_device_id"] = deviceID.String
		}
		if started.Valid {
			campaign["started_at"] = started.String
		}
		if completed.Valid {
			campaign["completed_at"] = completed.String
		}

		campaigns = append(campaigns, campaign)
	}

	c.JSON(http.StatusOK, gin.H{
		"campaigns": campaigns,
		"total":     len(campaigns),
	})
}

// CreateCampaign godoc
// @Summary Create OTA campaign
// @Description Create a new OTA update campaign targeting a fleet
// @Tags OTA
// @Accept json
// @Produce json
// @Param campaign body map[string]interface{} true "Campaign creation payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {string} string "Invalid request"
// @Failure 409 {string} string "Campaign ID already exists"
// @Router /api/v1/admin/ota/campaigns [post]
func (h *OTAHandler) CreateCampaign(c *gin.Context) {
	var req struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ArtifactID   string `json:"artifact_id"`
		TargetFleet  string `json:"target_fleet_id"`
		TargetDevice string `json:"target_device_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ID == "" || req.Name == "" || req.ArtifactID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID, name, and artifact_id are required"})
		return
	}

	_, err := h.db.Exec(`
		INSERT INTO ota_campaigns (id, name, artifact_id, target_fleet_id, target_device_id, status) 
		VALUES (?, ?, ?, ?, ?, 'draft')`, req.ID, req.Name, req.ArtifactID,
		sql.NullString{String: req.TargetFleet, Valid: req.TargetFleet != ""},
		sql.NullString{String: req.TargetDevice, Valid: req.TargetDevice != ""})

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.String(http.StatusConflict, "Campaign ID already exists")
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	h.logEvent("campaign_created", "campaign", req.ID, fmt.Sprintf(`{"name":"%s","artifact_id":"%s"}`, req.Name, req.ArtifactID))

	c.JSON(http.StatusCreated, gin.H{
		"id":     req.ID,
		"name":   req.Name,
		"status": "draft",
	})
}

// GetCampaign godoc
// @Summary Get campaign details
// @Description Get detailed information about a specific OTA campaign
// @Tags OTA
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {string} string "Campaign not found"
// @Router /api/v1/admin/ota/campaigns/{id} [get]
func (h *OTAHandler) GetCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	var id, name, artifactID, status, created string
	var fleetID, deviceID, started, completed sql.NullString
	var successCount, failureCount int

	err := h.db.QueryRow(`
		SELECT id, name, artifact_id, target_fleet_id, target_device_id, status, created_at, started_at, completed_at, success_count, failure_count 
		FROM ota_campaigns WHERE id = ?`, campaignID).
		Scan(&id, &name, &artifactID, &fleetID, &deviceID, &status, &created, &started, &completed, &successCount, &failureCount)

	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Campaign not found")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	campaign := gin.H{
		"id":            id,
		"name":          name,
		"artifact_id":   artifactID,
		"status":        status,
		"created_at":    created,
		"success_count": successCount,
		"failure_count": failureCount,
	}

	if fleetID.Valid {
		campaign["target_fleet_id"] = fleetID.String
	}
	if deviceID.Valid {
		campaign["target_device_id"] = deviceID.String
	}
	if started.Valid {
		campaign["started_at"] = started.String
	}
	if completed.Valid {
		campaign["completed_at"] = completed.String
	}

	c.JSON(http.StatusOK, campaign)
}

// StartCampaign godoc
// @Summary Start OTA campaign
// @Description Start an OTA campaign and begin rolling out updates
// @Tags OTA
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Campaign already started or invalid state"
// @Router /api/v1/admin/ota/campaigns/{id}/start [post]
func (h *OTAHandler) StartCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	result, err := h.db.Exec(`
		UPDATE ota_campaigns 
		SET status = 'active', started_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND status = 'draft'`, campaignID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign not found or already started"})
		return
	}

	h.logEvent("campaign_started", "campaign", campaignID, "{}")

	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// PauseCampaign godoc
// @Summary Pause OTA campaign
// @Description Pause an active OTA campaign
// @Tags OTA
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Campaign not active"
// @Router /api/v1/admin/ota/campaigns/{id}/pause [put]
func (h *OTAHandler) PauseCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	result, err := h.db.Exec(`
		UPDATE ota_campaigns 
		SET status = 'paused' 
		WHERE id = ? AND status = 'active'`, campaignID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign not found or not active"})
		return
	}

	h.logEvent("campaign_paused", "campaign", campaignID, "{}")

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// ResumeCampaign godoc
// @Summary Resume OTA campaign
// @Description Resume a paused OTA campaign
// @Tags OTA
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Campaign not paused"
// @Router /api/v1/admin/ota/campaigns/{id}/resume [put]
func (h *OTAHandler) ResumeCampaign(c *gin.Context) {
	campaignID := c.Param("id")

	result, err := h.db.Exec(`
		UPDATE ota_campaigns 
		SET status = 'active' 
		WHERE id = ? AND status = 'paused'`, campaignID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign not found or not paused"})
		return
	}

	h.logEvent("campaign_resumed", "campaign", campaignID, "{}")

	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// InstantDeploy godoc
// @Summary Rapidly deploy artifact to device
// @Description Creates and starts an OTA campaign for a specific device immediately
// @Tags OTA
// @Accept json
// @Produce json
// @Param deploy body map[string]interface{} true "Deploy payload"
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/dev/deploy [post]
func (h *OTAHandler) InstantDeploy(c *gin.Context) {
	var req struct {
		DeviceID   string `json:"device_id"`
		ArtifactID string `json:"artifact_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	campaignID := fmt.Sprintf("instant-%d", time.Now().UnixNano())
	campaignName := "Instant Flash: " + req.ArtifactID

	// 1. Create campaign already active
	_, err := h.db.Exec(`
		INSERT INTO ota_campaigns (id, name, artifact_id, target_device_id, status, started_at) 
		VALUES (?, ?, ?, ?, 'active', CURRENT_TIMESTAMP)`, campaignID, campaignName, req.ArtifactID, req.DeviceID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create instant campaign: " + err.Error()})
		return
	}

	h.logEvent("instant_deploy_started", "device", req.DeviceID, fmt.Sprintf(`{"artifact":"%s"}`, req.ArtifactID))

	c.JSON(http.StatusCreated, gin.H{
		"campaign_id": campaignID,
		"status":      "active",
		"message":     "Instant deployment started",
	})
}

func (h *OTAHandler) logEvent(eventType, resourceType, resourceID, payload string) {
	h.db.Exec(`INSERT INTO events (event_type, resource_type, resource_id, payload) VALUES (?, ?, ?, ?)`,
		eventType, resourceType, resourceID, payload)
}
