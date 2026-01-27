package registry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CheckUpdateRequest represents the query parameters for update check
type CheckUpdateRequest struct {
	DeviceID       string `json:"device_id"`
	CurrentVersion string `json:"current_version"`
}

// CheckUpdateResponse represents the response for update check
type CheckUpdateResponse struct {
	HasUpdate     bool   `json:"has_update"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
}

// CheckUpdate handles the heartbeat check-update endpoint
// GET /api/v1/registry/check-update?device_id=xxx&current_version=yyy
func (h *RegistryHandler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	currentVersion := r.URL.Query().Get("current_version")

	if deviceID == "" || currentVersion == "" {
		http.Error(w, "Missing device_id or current_version", http.StatusBadRequest)
		return
	}

	// SECURITY: Ensure device is registered
	if !h.Devices.IsApproved(deviceID) {
		fmt.Printf("⚠️  Registry: REJECTED update request from unapproved device %s\n", deviceID)
		http.Error(w, "Device not registered in this registry", http.StatusUnauthorized)
		return
	}

	// 3. Update telemetry of activity
	h.Devices.UpdateSeen(deviceID)

	// 4. Query for Active OTA Campaigns
	// First, get the device's fleet
	var fleetID sql.NullString
	err := h.Devices.db.QueryRow("SELECT fleet_id FROM devices WHERE id = ?", deviceID).Scan(&fleetID)
	if err != nil {
		// Device might have been deleted mid-request
		http.Error(w, "Device data corrupted", http.StatusInternalServerError)
		return
	}

	// Now check for an active campaign for this fleet OR this specific device
	var artifactID string
	var latestVersion string
	query := `
		SELECT c.artifact_id, a.version 
		FROM ota_campaigns c
		JOIN artifacts a ON c.artifact_id = a.id
		WHERE c.status = 'active' 
		AND (c.target_device_id = ? OR (c.target_device_id IS NULL AND (c.target_fleet_id = ? OR c.target_fleet_id IS NULL)))
		ORDER BY 
			CASE WHEN c.target_device_id = ? THEN 0 ELSE 1 END,
			c.created_at DESC 
		LIMIT 1`

	err = h.Devices.db.QueryRow(query, deviceID, fleetID, deviceID).Scan(&artifactID, &latestVersion)

	hasUpdate := false
	downloadURL := ""

	if err == nil {
		// We found a campaign! Now compare versions.
		if compareVersions(currentVersion, latestVersion) < 0 {
			hasUpdate = true
			baseURL, _ := r.Context().Value("BaseURL").(string)
			downloadURL = fmt.Sprintf("%s/api/v1/registry/download?id=%s", baseURL, artifactID)
		}
	}

	response := CheckUpdateResponse{
		HasUpdate:     hasUpdate,
		LatestVersion: latestVersion,
		DownloadURL:   downloadURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// compareVersions compares two semantic versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Simple string comparison for MVP
	// In production, use proper semver library
	if v1 == v2 {
		return 0
	}
	if strings.Compare(v1, v2) < 0 {
		return -1
	}
	return 1
}
