package api

import (
	"net/http"

	starter "vercel-go-starter"
	apphandler "vercel-go-starter/internal/handler"
)

var (
	h   = apphandler.New(starter.StaticFiles)
	mux = http.NewServeMux()
)

func init() {
	h.RegisterRoutes(mux)
}

// Handler is the Vercel serverless entry point.
// It delegates to the registered ServeMux which routes /api/sync, /api/data, etc.
func Handler(w http.ResponseWriter, r *http.Request) {
	mux.ServeHTTP(w, r)
}
