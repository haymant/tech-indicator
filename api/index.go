package api

import (
	"log/slog"
	"net/http"
	"os"

	starter "vercel-go-starter"
	apphandler "vercel-go-starter/internal/handler"
	"vercel-go-starter/internal/mcp"

	"vercel-go-starter/internal/database"
)

var (
	h   = apphandler.New(starter.StaticFiles)
	mux = http.NewServeMux()
)

func init() {
	// Run database migrations at startup.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		if err := database.RunMigrations(dbURL); err != nil {
			slog.Error("Database migration failed", "error", err)
		} else {
			slog.Info("Database migrations completed")
		}
	}

	h.RegisterRoutes(mux)
	mux.Handle("/api/mcp", mcp.NewHandler())
}

// Handler is the Vercel serverless entry point.
// It delegates to the registered ServeMux which routes /api/sync, /api/data, etc.
func Handler(w http.ResponseWriter, r *http.Request) {
	mux.ServeHTTP(w, r)
}
