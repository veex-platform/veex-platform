package vdl

import (
	"fmt"
	"path/filepath"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v2"
)

// ValidationError represents a problem found in the VDL
type ValidationError struct {
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Level   string `json:"level"` // "error", "warning"
}

// AllowedCapabilities defines the whitelist of valid capabilities
// We expand this to include more sensors as requested
var AllowedCapabilities = map[string]bool{
	"platform.core":        true,
	"platform.gpio":        true,
	"platform.i2c":         true,
	"platform.spi":         true,
	"platform.uart":        true,
	"platform.sensor":      true,
	"comm.mqtt":            true,
	"comm.http":            true,
	"comm.grpc":            true,
	"comm.can":             true,
	"comm.ble":             true,
	"sensor.bme280":        true,
	"sensor.accelerometer": true,
	"sensor.aht20":         true,
	"sensor.sht3x":         true,
	"modbus":               true,
	"ml":                   true,
	"logic":                true,
}

// SchemaPath is the relative path to the official VDL schema (from project root)
const SchemaPath = "../veex-build/internal/schema/vdl-v1.schema.json"

// ValidateVDL performs static analysis on the VDL string
func ValidateVDL(vdlString string) ([]ValidationError, error) {
	var validationErrors []ValidationError

	// 1. YAML Syntax Check (Mandatory before Schema check if parsing YAML to interface{})
	var vdlData interface{}
	if err := yaml.Unmarshal([]byte(vdlString), &vdlData); err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Message: fmt.Sprintf("YAML Syntax Error: %v", err),
			Level:   "error",
		})
		return validationErrors, nil
	}

	// 2. Normalize YAML to JSON-compatible types for JSON Schema validation
	// (yaml.v2 unmarshals maps as map[interface{}]interface{}, but gojsonschema needs map[string]interface{})
	jsonCompatibleData := convertMapIToMapS(vdlData)

	// 3. JSON Schema Validation
	schemaAbsPath, _ := filepath.Abs(SchemaPath)
	schemaLoader := gojsonschema.NewReferenceLoader("file:///" + filepath.ToSlash(schemaAbsPath))
	documentLoader := gojsonschema.NewGoLoader(jsonCompatibleData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("schema validation system error: %v", err)
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			validationErrors = append(validationErrors, ValidationError{
				Message: fmt.Sprintf("[Schema] %s", desc.String()),
				Level:   "error",
			})
		}
	}

	// 4. Semantic Validation (Capabilities Whitelist)
	vdlMap, ok := jsonCompatibleData.(map[string]interface{})
	if ok {
		if flows, ok := vdlMap["flows"].([]interface{}); ok {
			for i, flow := range flows {
				if f, ok := flow.(map[string]interface{}); ok {
					if steps, ok := f["steps"].([]interface{}); ok {
						validateSemanticSteps(steps, &validationErrors, fmt.Sprintf("flows[%d]", i))
					}
				}
			}
		}
	}

	return validationErrors, nil
}

func validateSemanticSteps(steps []interface{}, errors *[]ValidationError, context string) {
	for _, step := range steps {
		s, ok := step.(map[string]interface{})
		if !ok {
			continue
		}

		capability := fmt.Sprintf("%v", s["capability"])
		if capability != "" && !AllowedCapabilities[capability] {
			*errors = append(*errors, ValidationError{
				Message: fmt.Sprintf("[%s] Unknown capability: '%s'", context, capability),
				Level:   "error",
			})
		}
	}
}

// Convert map[interface{}]interface{} to map[string]interface{} recursively
func convertMapIToMapS(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[fmt.Sprintf("%v", k)] = convertMapIToMapS(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convertMapIToMapS(v)
		}
	}
	return i
}
