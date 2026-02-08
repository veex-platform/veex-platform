package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/veex-platform/veex-platform/internal/vdl"
)

type ValidationRequest struct {
	VDL string `json:"vdl"`
}

type ValidationResponse struct {
	Valid  bool                  `json:"valid"`
	Errors []vdl.ValidationError `json:"errors"`
}

// ValidateVDL godoc
// @Summary Validate VDL syntax and structure
// @Description Performs static analysis on the provided VDL code
// @Tags Developer
// @Accept json
// @Produce json
// @Param validation body ValidationRequest true "VDL Code"
// @Success 200 {object} ValidationResponse
// @Router /api/v1/dev/validate [post]
func ValidateVDLHandler(c *gin.Context) {
	var req ValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	errors, err := vdl.ValidateVDL(req.VDL)
	if err != nil {
		// Should not usually happen as ValidateVDL returns errors slice
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ValidationResponse{
		Valid:  len(errors) == 0,
		Errors: errors,
	})
}
