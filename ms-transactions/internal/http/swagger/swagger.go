package swagger

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
	httpswagger "github.com/swaggo/http-swagger/v2"
)

//go:embed openapi.yaml
var spec []byte

// Register mounts the Swagger UI and OpenAPI spec under /swagger.
// Only call this when APP_ENV=Development.
func Register(r chi.Router) {
	r.Get("/swagger/openapi.yaml", serveSpec)
	r.Get("/swagger", http.RedirectHandler("/swagger/", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/swagger/*", httpswagger.Handler(
		httpswagger.URL("/swagger/openapi.yaml"),
	))
}

func serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(spec) //nolint:errcheck
}
