package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func InitDB(dataPath string) (*sql.DB, error) {
	dbPath := filepath.Join(dataPath, "veex.db")

	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Create Tables with Industrial-Scale Optimizations
	schema := `
	-- Device Registry (millions of rows expected)
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		firmware_version TEXT,
		fleet_id TEXT,
		metadata TEXT  -- JSON for extensibility
	);

	-- Index for sorting by activity (dashboard queries)
	CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen DESC);
	CREATE INDEX IF NOT EXISTS idx_devices_fleet ON devices(fleet_id);

	-- Artifacts Registry
	CREATE TABLE IF NOT EXISTS artifacts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		target_arch TEXT,
		file_name TEXT,
		download_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_artifacts_name ON artifacts(name);
	CREATE INDEX IF NOT EXISTS idx_artifacts_version ON artifacts(version);

	-- Device Fleets/Groups
	CREATE TABLE IF NOT EXISTS fleets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- OTA Campaigns
	CREATE TABLE IF NOT EXISTS ota_campaigns (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		artifact_id TEXT NOT NULL,
		target_fleet_id TEXT,
		status TEXT DEFAULT 'draft', -- draft, active, paused, completed, failed
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME,
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		FOREIGN KEY (target_fleet_id) REFERENCES fleets(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_campaigns_status ON ota_campaigns(status);
	CREATE INDEX IF NOT EXISTS idx_campaigns_fleet ON ota_campaigns(target_fleet_id);

	-- Events/Audit Log
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL, -- device_registered, firmware_updated, campaign_started, etc
		resource_type TEXT,       -- device, fleet, campaign, artifact
		resource_id TEXT,
		payload TEXT,             -- JSON
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
	CREATE INDEX IF NOT EXISTS idx_events_resource ON events(resource_type, resource_id);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);

	-- Telemetry Signals (billions of rows expected over time)
	CREATE TABLE IF NOT EXISTS signals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		source TEXT,
		signal_type TEXT,  -- e.g., 'cpu_temp', 'battery', 'error_log'
		payload TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	-- Critical: Index for device-specific queries (80% of queries)
	CREATE INDEX IF NOT EXISTS idx_signals_device_time ON signals(device_id, timestamp DESC);
	
	-- Critical: Index for time-based aggregations (dashboard, analytics)
	CREATE INDEX IF NOT EXISTS idx_signals_timestamp ON signals(timestamp DESC);

	-- Optional: Index for signal-type filtering (if needed)
	CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(signal_type);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %v", err)
	}

	fmt.Printf("🗄️  SQLite: Database initialized at %s\n", dbPath)
	return db, nil
}
