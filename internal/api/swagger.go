package api

import (
	"embed"
	"net/http"
	"strings"
)

var apiSpec embed.FS

type SwaggerHandler struct{}

func NewSwaggerHandler() *SwaggerHandler {
	return &SwaggerHandler{}
}

// Serve Swagger UI
func (h *SwaggerHandler) ServeUI(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>VEEX Platform API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "/api/docs/openapi.yaml",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                tryItOutEnabled: true
            });
        };
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Serve OpenAPI spec
func (h *SwaggerHandler) ServeSpec(w http.ResponseWriter, r *http.Request) {
	// Check if requesting YAML
	if strings.HasSuffix(r.URL.Path, ".yaml") || strings.HasSuffix(r.URL.Path, ".yml") {
		spec, err := apiSpec.ReadFile("openapi.yaml")
		if err != nil {
			http.Error(w, "Spec not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(spec)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}
