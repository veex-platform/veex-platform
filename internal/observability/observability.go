package observability

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TelemetrySignal struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
	Signal    string    `json:"signal"`
	Value     string    `json:"value"`
}

type ObsHandler struct {
	db *sql.DB
}

func NewObsHandler(database *sql.DB) *ObsHandler {
	return &ObsHandler{db: database}
}

func (h *ObsHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var signal TelemetrySignal
	if err := json.NewDecoder(r.Body).Decode(&signal); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := `INSERT INTO signals (device_id, source, payload, timestamp) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`
	if _, err := h.db.Exec(query, signal.DeviceID, "http-api", signal.Signal+"="+signal.Value); err != nil {
		fmt.Printf("❌ [OBS] Database error: %v\n", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	fmt.Printf("📊 [OBS] Signal persisted in SQLite from %s\n", signal.DeviceID)
	w.WriteHeader(http.StatusAccepted)
}

func (h *ObsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	rows, err := h.db.Query("SELECT device_id, payload, timestamp FROM signals ORDER BY timestamp DESC LIMIT 100")
	if err != nil {
		http.Error(w, "Query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	fmt.Fprintf(w, "<html><head><meta http-equiv='refresh' content='5'><title>VEEX Observability</title>")
	fmt.Fprintf(w, "<style>body{background:#0f172a;color:#94a3b8;font-family:sans-serif;padding:40px;}")
	fmt.Fprintf(w, "h1{color:#38bdf8;} .card{background:#1e293b;padding:15px;margin:10px 0;border-radius:8px;border:1px solid #334155;}")
	fmt.Fprintf(w, ".ts{color:#64748b;font-size:12px;} .signal{color:#fbbf24;font-weight:bold;}</style></head><body>")
	fmt.Fprintf(w, "<h1>📊 VEEX SQL Industrial Dashboard</h1>")

	count := 0
	for rows.Next() {
		var devID, payload, ts string
		rows.Scan(&devID, &payload, &ts)
		fmt.Fprintf(w, "<div class='card'><span class='ts'>%s</span> | Device: <b>%s</b> | Data: <span class='signal'>%s</span></div>",
			ts, devID, payload)
		count++
	}

	if count == 0 {
		fmt.Fprintf(w, "<p>No industrial signals found in database.</p>")
	}

	fmt.Fprintf(w, "</body></html>")
}
