// Package apiserver provides OpenAPI documentation handling
package apiserver

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

//go:embed openapi.yaml
var openAPISpec embed.FS

// OpenAPIHandler provides OpenAPI/Swagger documentation endpoints
type OpenAPIHandler struct {
	logger *zap.Logger
	spec   string
}

// NewOpenAPIHandler creates a new OpenAPI handler
func NewOpenAPIHandler(logger *zap.Logger) *OpenAPIHandler {
	// Read the embedded OpenAPI spec
	specData, err := openAPISpec.ReadFile("openapi.yaml")
	if err != nil {
		logger.Error("Failed to read OpenAPI spec", zap.Error(err))
		return &OpenAPIHandler{
			logger: logger,
			spec:   "# OpenAPI spec not available",
		}
	}

	return &OpenAPIHandler{
		logger: logger,
		spec:   string(specData),
	}
}

// ServeOpenAPISpec serves the OpenAPI specification in YAML format
func (h *OpenAPIHandler) ServeOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(h.spec))
}

// ServeOpenAPIJSON serves the OpenAPI specification in JSON format
func (h *OpenAPIHandler) ServeOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	
	// Simple JSON encoding for the response
	fmt.Fprintf(w, `{
		"openapi": "3.0.3",
		"info": {
			"title": "Alchemorsel API v3 - Pure Backend",
			"description": "Enterprise-grade recipe management API with AI capabilities",
			"version": "3.0.0"
		},
		"servers": [
			{"url": "%s://%s/api/v1", "description": "Current server"}
		],
		"spec_url": "%s://%s/api/v1/openapi.yaml",
		"docs_url": "%s://%s/api/v1/docs"
	}`, getScheme(r), r.Host, getScheme(r), r.Host, getScheme(r), r.Host)
}

// ServeSwaggerUI serves a basic Swagger UI interface
func (h *OpenAPIHandler) ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	specURL := fmt.Sprintf("%s://%s/api/v1/openapi.yaml", getScheme(r), r.Host)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Alchemorsel API v3 Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
        .topbar {
            background: #1f2937 !important;
        }
        .topbar .download-url-wrapper {
            display: none;
        }
        .swagger-ui .topbar .topbar-wrapper::before {
            content: "🍽 Alchemorsel v3 API";
            color: white;
            font-size: 1.5em;
            font-weight: bold;
            margin-right: 20px;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: '%s',
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
                tryItOutEnabled: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                validatorUrl: null,
                docExpansion: 'list',
                operationsSorter: 'alpha',
                tagsSorter: 'alpha',
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                displayRequestDuration: true,
                requestInterceptor: function(request) {
                    // Add custom headers or modify requests here
                    console.log('Request:', request);
                    return request;
                },
                responseInterceptor: function(response) {
                    console.log('Response:', response);
                    return response;
                }
            });
            
            // Custom styling
            setTimeout(function() {
                const info = document.querySelector('.info');
                if (info) {
                    info.style.marginTop = '20px';
                }
                
                // Add custom banner
                const wrapper = document.querySelector('.swagger-ui');
                if (wrapper && !document.querySelector('.custom-banner')) {
                    const banner = document.createElement('div');
                    banner.className = 'custom-banner';
                    banner.style.cssText = 'background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 20px; text-align: center; margin-bottom: 20px; border-radius: 8px; margin: 20px;';
                    banner.innerHTML = '<h2 style="margin: 0 0 10px 0;">🍽 Alchemorsel v3 API Documentation</h2>' +
                        '<p style="margin: 0; opacity: 0.9;">Enterprise-grade recipe management with AI capabilities</p>' +
                        '<div style="margin-top: 15px; font-size: 0.9em;">' +
                        '<span style="background: rgba(255,255,255,0.2); padding: 4px 8px; border-radius: 4px; margin: 0 5px;">Pure JSON API</span>' +
                        '<span style="background: rgba(255,255,255,0.2); padding: 4px 8px; border-radius: 4px; margin: 0 5px;">OpenAPI 3.0</span>' +
                        '<span style="background: rgba(255,255,255,0.2); padding: 4px 8px; border-radius: 4px; margin: 0 5px;">JWT Auth</span>' +
                        '</div>';
                    wrapper.insertBefore(banner, wrapper.firstChild);
                }
            }, 1000);
        };
    </script>
</body>
</html>`, specURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// ServeRedocUI serves a Redoc UI interface (alternative to Swagger UI)
func (h *OpenAPIHandler) ServeRedocUI(w http.ResponseWriter, r *http.Request) {
	specURL := fmt.Sprintf("%s://%s/api/v1/openapi.yaml", getScheme(r), r.Host)
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Alchemorsel API v3 Documentation</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
    <style>
        body {
            margin: 0;
            padding: 0;
        }
        .custom-header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px;
            text-align: center;
        }
        .custom-header h1 {
            margin: 0 0 10px 0;
            font-family: 'Montserrat', sans-serif;
        }
        .custom-header p {
            margin: 0;
            opacity: 0.9;
            font-family: 'Roboto', sans-serif;
        }
    </style>
</head>
<body>
    <div class="custom-header">
        <h1>🍽 Alchemorsel v3 API</h1>
        <p>Enterprise-grade recipe management with AI capabilities</p>
    </div>
    <redoc spec-url='%s'></redoc>
    <script src="https://cdn.redoc.ly/redoc/2.1.3/bundles/redoc.standalone.js"></script>
</body>
</html>`, specURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// getScheme determines the URL scheme (http/https) from the request
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	
	// Check forwarded headers for proxy setups
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	
	if strings.Contains(r.Host, "localhost") || strings.Contains(r.Host, "127.0.0.1") {
		return "http"
	}
	
	return "http" // Default to http for development
}