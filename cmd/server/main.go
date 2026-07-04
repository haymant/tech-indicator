package main

import (
	"log/slog"
	"net/http"
	"os"

	starter "vercel-go-starter"
	"vercel-go-starter/internal/database"
	"vercel-go-starter/internal/handler"
	"vercel-go-starter/internal/mcp"
)

func main() {
	// Run database migrations at startup.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		if err := database.RunMigrations(dbURL); err != nil {
			slog.Error("Database migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Database migrations completed")
	}

	mux := http.NewServeMux()

	h := handler.New(starter.StaticFiles)
	h.RegisterRoutes(mux)

	// Register MCP endpoint.
	mux.Handle("/api/mcp", mcp.NewHandler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	slog.Info("Server starting", "port", port)
	http.ListenAndServe(":"+port, mux)
}
