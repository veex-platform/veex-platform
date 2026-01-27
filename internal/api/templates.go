package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type TemplateEntry struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	VDL         string `json:"vdl"`
}

type TemplatesHandler struct {
	TemplatesDir string
}

func NewTemplatesHandler(templatesDir string) *TemplatesHandler {
	return &TemplatesHandler{TemplatesDir: templatesDir}
}

// ListTemplates scans the templates directory and returns all VDL templates
// @Summary List industrial templates
// @Description Scans the templates directory and returns all VDL templates
// @Tags Registry
// @Produce json
// @Success 200 {array} TemplateEntry
// @Router /api/v1/dev/templates [get]
func (h *TemplatesHandler) ListTemplates(c *gin.Context) {
	templates := []TemplateEntry{}

	// Walk through the templates directory
	err := filepath.Walk(h.TemplatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for .vdl files
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".vdl") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files we can't read
			}

			// Extract category from parent directory name
			category := filepath.Base(filepath.Dir(path))

			// Simple ID generation
			id := strings.TrimSuffix(info.Name(), ".vdl")

			// Create a human-friendly title
			title := strings.Title(strings.ReplaceAll(id, "-", " "))

			templates = append(templates, TemplateEntry{
				ID:          id,
				Title:       title,
				Description: fmt.Sprintf("Industrial template for %s", category),
				Category:    category,
				VDL:         string(content),
			})
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan templates"})
		return
	}

	c.JSON(http.StatusOK, templates)
}
