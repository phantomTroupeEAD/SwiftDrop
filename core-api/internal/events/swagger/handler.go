package swagger

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// RegisterSwaggerRoutes registers the Swagger UI and index endpoints on the multiplexer.
func RegisterSwaggerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /swagger/", handleSwaggerIndex)
	mux.HandleFunc("GET /swagger/{name}/", handleSwaggerUI)
	mux.HandleFunc("GET /swagger/{name}/spec.json", handleSwaggerSpec)
	mux.HandleFunc("GET /api/v1/events/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if spec, ok := GetSpec("events"); ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(spec)
			return
		}
		http.NotFound(w, r)
	})
}

func handleSwaggerIndex(w http.ResponseWriter, r *http.Request) {
	// If path has subsegment like /swagger/events/, it should route to handleSwaggerUI
	path := strings.TrimPrefix(r.URL.Path, "/swagger/")
	if path != "" && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	specs := ListSpecs()
	var names []string
	for k := range specs {
		names = append(names, k)
	}
	sort.Strings(names)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>SwiftDrop API Specifications</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 40px; line-height: 1.6; background: #f8fafc; color: #1e293b; }
        .card { background: white; border: 1px solid #e2e8f0; border-radius: 8px; padding: 24px; max-width: 600px; margin: 0 auto; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1); }
        h1 { margin-top: 0; font-size: 24px; color: #0f172a; border-bottom: 2px solid #e2e8f0; padding-bottom: 12px; }
        ul { list-style: none; padding: 0; }
        li { padding: 12px 0; border-bottom: 1px solid #f1f5f9; display: flex; justify-content: space-between; align-items: center; }
        li:last-child { border-bottom: none; }
        a.btn { background: #2563eb; color: white; text-decoration: none; padding: 6px 14px; border-radius: 6px; font-weight: 500; font-size: 14px; }
        a.btn:hover { background: #1d4ed8; }
        .meta { color: #64748b; font-size: 13px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="card">
        <h1>SwiftDrop OpenAPI Directory</h1>
        <p>Interactive documentation for SwiftDrop API modules:</p>
        <ul>`)

	for _, name := range names {
		fmt.Fprintf(w, `
            <li>
                <strong>%s API</strong>
                <a class="btn" href="/swagger/%s/">Explore UI &rarr;</a>
            </li>`, strings.Title(name), name)
	}

	fmt.Fprintf(w, `
        </ul>
        <div class="meta">
            SwiftDrop Modular Monolith &bull; Core API Gateway &bull; Go 1.22
        </div>
    </div>
</body>
</html>`)
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.NotFound(w, r)
		return
	}

	spec, ok := GetSpec(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = spec

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
            url: "/swagger/%s/spec.json",
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
</html>`, strings.Title(name), name)
}

func handleSwaggerSpec(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	spec, ok := GetSpec(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(spec)
}
