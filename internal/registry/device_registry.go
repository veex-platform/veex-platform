package registry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DeviceInfo struct {
	ID         string    `json:"id"`
	Registered bool      `json:"registered"`
	LastSeen   time.Time `json:"last_seen"`
}

type DeviceRegistry struct {
	db *sql.DB
}

func NewDeviceRegistry(database *sql.DB) *DeviceRegistry {
	return &DeviceRegistry{db: database}
}

func (r *DeviceRegistry) Register(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var device DeviceInfo
	if err := json.NewDecoder(req.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	query := `INSERT INTO devices (id) VALUES (?) ON CONFLICT(id) DO UPDATE SET last_seen=CURRENT_TIMESTAMP`
	if _, err := r.db.Exec(query, device.ID); err != nil {
		fmt.Printf("❌ Database error during registration: %v\n", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	device.Registered = true
	device.LastSeen = time.Now()

	fmt.Printf("🛡️  Registry: Device %s registered and trusted in SQLite.\n", device.ID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(device)
}

func (r *DeviceRegistry) IsApproved(id string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)`
	err := r.db.QueryRow(query, id).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (r *DeviceRegistry) UpdateSeen(id string) {
	query := `UPDATE devices SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`
	r.db.Exec(query, id)
}
