package swagger

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"
)

//go:embed swagger.json
var UnifiedSwaggerJSON []byte

// RegisterSwaggerRoutes registers the Swagger UI and spec endpoints on the multiplexer.
func RegisterSwaggerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /swagger/", handleSwaggerUnifiedUI)
	mux.HandleFunc("GET /swagger/spec.json", handleUnifiedSpec)
	mux.HandleFunc("GET /swagger/swagger.json", handleUnifiedSpec)
	mux.HandleFunc("GET /api/v1/openapi.json", handleUnifiedSpec)
	mux.HandleFunc("GET /swagger/{name}/", handleSwaggerUI)
	mux.HandleFunc("GET /swagger/{name}/spec.json", handleSwaggerSpec)
	mux.HandleFunc("GET /api/v1/events/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if spec, ok := GetSpec("events"); ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(spec)
			return
		}
		if len(UnifiedSwaggerJSON) > 0 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(UnifiedSwaggerJSON)
			return
		}
		http.NotFound(w, r)
	})
}

func handleUnifiedSpec(w http.ResponseWriter, _ *http.Request) {
	if len(UnifiedSwaggerJSON) == 0 {
		http.Error(w, `{"error":"swagger spec not loaded"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(UnifiedSwaggerJSON)
}

func handleSwaggerUnifiedUI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/swagger/")
	if path != "" && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	// If a subsegment like /swagger/events/ was passed through, delegate to module UI
	if path != "" {
		r.SetPathValue("name", strings.TrimSuffix(path, "/"))
		handleSwaggerUI(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>SwiftDrop Core API Documentation</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; background: #fafafa; }
        .topbar { display: none !important; }
        .swagger-ui .info { margin: 20px 0; }
        .swagger-ui .info .title { font-size: 28px; color: #0f172a; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-standalone-preset.js"></script>
    <script>
    window.onload = function() {
        SwaggerUIBundle({
            url: "/swagger/spec.json",
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout",
            docExpansion: "list",
            filter: true
        });
    };
    </script>
</body>
</html>`)
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		handleSwaggerUnifiedUI(w, r)
		return
	}

	specURL := fmt.Sprintf("/swagger/%s/spec.json", name)
	if _, ok := GetSpec(name); !ok {
		// If specific module not found, fallback to unified spec
		specURL = "/swagger/spec.json"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>%s API Documentation - SwiftDrop</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; }
        .topbar { display: none !important; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-bundle.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.18.2/swagger-ui-standalone-preset.js"></script>
    <script>
    window.onload = function() {
        SwaggerUIBundle({
            url: "%s",
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout"
        });
    };
    </script>
</body>
</html>`, toTitle(name), specURL)
}

func toTitle(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func handleSwaggerSpec(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if spec, ok := GetSpec(name); ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(spec)
		return
	}
	if len(UnifiedSwaggerJSON) > 0 {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(UnifiedSwaggerJSON)
		return
	}
	http.NotFound(w, r)
}
