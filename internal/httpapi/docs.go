package httpapi

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/souzavinny/reviews-api/api"
)

//go:embed all:swaggerui
var swaggerUI embed.FS

// mountDocs serves the OpenAPI spec at /openapi.yaml and an embedded Swagger UI
// at /docs — both offline, no third-party Go dependency.
func mountDocs(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		if _, err := w.Write(api.Spec); err != nil {
			log.Printf("httpapi: write openapi spec: %v", err)
		}
	})

	ui, err := fs.Sub(swaggerUI, "swaggerui")
	if err != nil {
		panic(err) // the embedded directory is fixed at build time
	}
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServerFS(ui)))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
}
