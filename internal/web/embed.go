package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// GetStaticHandler returns an HTTP handler for static files
func GetStaticHandler() http.Handler {
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(subFS))
}
