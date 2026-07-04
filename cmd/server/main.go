package main

import (
	"net/http"
	"os"

	starter "vercel-go-starter"
	"vercel-go-starter/internal/handler"
	"vercel-go-starter/internal/mcp"
)

func main() {
	mux := http.NewServeMux()

	h := handler.New(starter.StaticFiles)
	h.RegisterRoutes(mux)

	// Register MCP endpoint.
	mux.Handle("/api/mcp", mcp.NewHandler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.ListenAndServe(":"+port, mux)
}
