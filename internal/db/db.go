package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func InitDB(dataPath string) (*sql.DB, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "sqlite"
	}

	var db *sql.DB
	var err error

	if dbType == "postgres" {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			return nil, fmt.Errorf("DATABASE_URL must be set when DB_TYPE is postgres")
		}
		db, err = sql.Open("postgres", dbURL)
		fmt.Println("🗄️  PostgreSQL: Connecting to external database...")
	} else {
		dbPath := filepath.Join(dataPath, "veex.db")
		// Create directory if it doesn't exist
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		db, err = sql.Open("sqlite", dbPath)
		fmt.Printf("🗄️  SQLite: Database initialized at %s\n", dbPath)
	}

	if err != nil {
		return nil, err
	}

	// Schema logic with dialect awareness
	isPostgres := dbType == "postgres"
	autoInc := "AUTOINCREMENT"
	if isPostgres {
		autoInc = "SERIAL"
	}

	// Helper for cross-dialect primary keys
	pkType := "INTEGER PRIMARY KEY " + autoInc
	if isPostgres {
		pkType = "SERIAL PRIMARY KEY"
	}

	schema := []string{
		`-- Device Registry
		CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			firmware_version TEXT,
			fleet_id TEXT,
			metadata TEXT
		);`,

		`CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_devices_fleet ON devices(fleet_id);`,

		`-- Artifacts Registry
		CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			target_arch TEXT,
			file_name TEXT,
			download_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_artifacts_name ON artifacts(name);`,
		`CREATE INDEX IF NOT EXISTS idx_artifacts_version ON artifacts(version);`,

		`-- Device Fleets
		CREATE TABLE IF NOT EXISTS fleets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`-- OTA Campaigns
		CREATE TABLE IF NOT EXISTS ota_campaigns (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			artifact_id TEXT NOT NULL,
			target_fleet_id TEXT,
			status TEXT DEFAULT 'draft',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			success_count INTEGER DEFAULT 0,
			failure_count INTEGER DEFAULT 0,
			FOREIGN KEY (target_fleet_id) REFERENCES fleets(id) ON DELETE SET NULL
		);`,

		`CREATE INDEX IF NOT EXISTS idx_campaigns_status ON ota_campaigns(status);`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_fleet ON ota_campaigns(target_fleet_id);`,

		fmt.Sprintf(`-- Events/Audit Log
		CREATE TABLE IF NOT EXISTS events (
			id %s,
			event_type TEXT NOT NULL,
			resource_type TEXT,
			resource_id TEXT,
			payload TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`, pkType),

		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);`,
		`CREATE INDEX IF NOT EXISTS idx_events_resource ON events(resource_type, resource_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);`,

		fmt.Sprintf(`-- Telemetry Signals
		CREATE TABLE IF NOT EXISTS signals (
			id %s,
			device_id TEXT NOT NULL,
			source TEXT,
			signal_type TEXT,
			payload TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		);`, pkType),

		`CREATE INDEX IF NOT EXISTS idx_signals_device_time ON signals(device_id, timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_signals_timestamp ON signals(timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(signal_type);`,
	}

	for _, stmt := range schema {
		// PostgreSQL DATETIME is TIMESTAMP
		if isPostgres {
			stmt = strings.ReplaceAll(stmt, "DATETIME", "TIMESTAMP")
		}
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("failed to execute schema statement: %v", err)
		}
	}

	return db, nil
}
