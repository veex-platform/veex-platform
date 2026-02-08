package api

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

// SimulationRequest godoc
type SimulationRequest struct {
	VDL     string                 `json:"vdl"`
	Payload map[string]interface{} `json:"payload"`
}

// LogEntry represents a single step in the simulation log
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	StepID    string `json:"step_id,omitempty"`
}

// SimulationResponse godoc
type SimulationResponse struct {
	Status string     `json:"status"`
	Logs   []LogEntry `json:"logs"`
	Final  string     `json:"final_state"`
}

// Simulate godoc
// @Summary Simulate VDL execution
// @Description Dry-run a VDL pipeline with provided inputs and return execution logs
// @Tags Developer
// @Accept json
// @Produce json
// @Param simulation body SimulationRequest true "VDL and Input Payload"
// @Success 200 {object} SimulationResponse
// @Router /api/v1/dev/simulate [post]
func (h *OTAHandler) Simulate(c *gin.Context) {
	var req SimulationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	logs := []LogEntry{}
	log := func(level, msg, stepID string) {
		logs = append(logs, LogEntry{
			Timestamp: time.Now().Format("15:04:05.000"),
			Level:     level,
			Message:   msg,
			StepID:    stepID,
		})
	}

	log("info", "Starting simulation...", "init")

	// 1. Parse VDL (Simple YAML validation)
	var vdlMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.VDL), &vdlMap); err != nil {
		log("error", "Failed to parse VDL: "+err.Error(), "parse")
		c.JSON(http.StatusOK, SimulationResponse{Status: "failed", Logs: logs})
		return
	}

	// Try to get name from root (VDL 1.0 might not have name in root, but let's check)
	if name, ok := vdlMap["name"].(string); ok {
		log("info", fmt.Sprintf("Loaded VDL: %s", name), "parse")
	} else {
		log("info", "Loaded VDL: [Unnamed Flow]", "parse")
	}

	// 2. Normalize Steps (Support both Legacy 'pipeline' and VDL 1.0 'flows.main_loop.steps')
	var steps []interface{}

	// Check for VDL 1.0 flows
	if flows, ok := vdlMap["flows"].(map[interface{}]interface{}); ok {
		if mainLoop, ok := flows["main_loop"].(map[interface{}]interface{}); ok {
			if s, ok := mainLoop["steps"].([]interface{}); ok {
				steps = s
				log("info", "Detected VDL 1.0 format", "parse")
			}
		}
	}

	// Fallback to Legacy 'pipeline'
	if steps == nil {
		if pipeline, ok := vdlMap["pipeline"].([]interface{}); ok {
			steps = pipeline
			log("info", "Detected Legacy VDL format", "parse")
		}
	}

	if len(steps) == 0 {
		log("warning", "No steps found in VDL", "parse")
	} else {
		for _, step := range steps {
			s := step.(map[interface{}]interface{})

			// Extract Step Metadata (Handle both formats)
			var id, capability, action string

			// VDL 1.0
			if val, ok := s["name"]; ok {
				id = fmt.Sprintf("%v", val)
			}
			if val, ok := s["capability"]; ok {
				capability = fmt.Sprintf("%v", val)
			}
			if val, ok := s["action"]; ok {
				action = fmt.Sprintf("%v", val)
			}

			// Legacy Polyfill
			if capability == "" {
				// Legacy 'type' field often combined capability/action (e.g. "modbus/read")
				if typ, ok := s["type"].(string); ok {
					parts := strings.Split(typ, "/")
					if len(parts) == 2 {
						capability = parts[0]
						action = parts[1]
					} else {
						capability = typ
					}
				}
				if val, ok := s["id"]; ok {
					id = fmt.Sprintf("%v", val)
				}
			}

			// Simulate processing delay
			time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

			log("info", fmt.Sprintf("Executing [%s] -> %s.%s", id, capability, action), id)

			// Mock specific behaviors
			switch {
			case capability == "modbus" || strings.HasPrefix(capability, "modbus"):
				log("debug", fmt.Sprintf("Reading register 4001: %d", 450+rand.Intn(50)), id)

			case capability == "ml" || strings.HasPrefix(capability, "ml"):
				score := 0.1 + rand.Float64()*0.85
				log("info", fmt.Sprintf("Inference Result: Anomaly Score = %.4f", score), id)
				if score > 0.8 {
					log("warn", "Anomaly detected! Triggering condition.", id)
				}

			case capability == "comm.ble" || strings.HasPrefix(capability, "ble"):
				if action == "advertise" {
					log("info", "BLE Advertising Started: VEEX-NODE", id)
				} else {
					log("info", "BLE Action executed", id)
				}

			case capability == "cloud" || capability == "comm.mqtt" || capability == "comm.http":
				log("success", "Data pushed to upstream successfully.", id)

			case capability == "platform.gpio":
				log("debug", "GPIO State Updated", id)
			}
		}
	}

	log("success", "Simulation completed successfully.", "end")

	c.JSON(http.StatusOK, SimulationResponse{
		Status: "success",
		Logs:   logs,
		Final:  "Simulation finished",
	})
}
