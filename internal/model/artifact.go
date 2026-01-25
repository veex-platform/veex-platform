package model

import "time"

type Artifact struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	TargetArch  string    `json:"target_arch"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	FileName    string    `json:"file_name"`
	DownloadURL string    `json:"download_url,omitempty"`
	HasSBOM     bool      `json:"has_sbom"`
}
