package handler

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"vercel-go-starter/internal/model"
	"vercel-go-starter/internal/repository"

	"github.com/cinar/indicator/v2/asset"
)

// SyncRunner abstracts the synchronous sync operation so it can be
// replaced in tests without requiring real Tiingo or MotherDuck connections.
type SyncRunner func(tiingoKey, databaseURL string, req model.SyncRequest) error

// DefaultSyncRunner is the production implementation that connects to real repositories.
func DefaultSyncRunner(tiingoKey, databaseURL string, req model.SyncRequest) error {

	source := asset.NewTiingoRepository(tiingoKey)

	target, err := asset.NewRepository(repository.MotherDuckRepositoryName, databaseURL)
	if err != nil {
		return err
	}

	sync := asset.NewSync()
	sync.Workers = req.Workers
	sync.Delay = req.Delay
	sync.Assets = req.Assets
	sync.Logger = slog.Default()

	defaultStartDate := time.Now().AddDate(0, 0, -req.Days)

	return sync.Run(source, target, defaultStartDate)
}

type Handler struct {
	assets   embed.FS
	syncFunc SyncRunner
}

func New(assets embed.FS) *Handler {
	repository.RegisterMotherDuck()
	return &Handler{
		assets:   assets,
		syncFunc: DefaultSyncRunner,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	publicFS, _ := fs.Sub(h.assets, "public")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(publicFS))))

	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/favicon.ico", h.handleFavicon)
	mux.HandleFunc("/api/data", h.handleGetData)
	mux.HandleFunc("/api/items/", h.handleGetItem)
	mux.HandleFunc("/api/sync", h.handleSync)
	mux.HandleFunc("/api/indicators", h.handleListIndicators)
	mux.HandleFunc("/api/indicators/calculate", h.handleCalculateIndicators)
	mux.HandleFunc("/api/indicators/values", h.handleGetIndicatorValues)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := h.assets.ReadFile("public/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (h *Handler) handleFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := h.assets.ReadFile("public/favicon.ico")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(data)
}

func (h *Handler) handleGetData(w http.ResponseWriter, r *http.Request) {
	items := []model.DataItem{
		{ID: 1, Name: "Sample Item 1", Value: 100},
		{ID: 2, Name: "Sample Item 2", Value: 200},
		{ID: 3, Name: "Sample Item 3", Value: 300},
	}

	writeJSON(w, http.StatusOK, model.DataResponse{
		Data:      items,
		Total:     len(items),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/items/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, model.ItemResponse{
		Item: model.DataItem{
			ID:    1,
			Name:  "Sample Item " + id,
			Value: 100,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status:    "error",
			Message:   "Method not allowed",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if !requireBearerAuth(r) {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{
			Status:    "error",
			Message:   "Unauthorized",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	tiingoKey := os.Getenv("TIINGO_API_KEY")
	if tiingoKey == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status:    "error",
			Message:   "TIINGO_API_KEY not set",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status:    "error",
			Message:   "DATABASE_URL not set",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	var req model.SyncRequest
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Status:    "error",
					Message:   "Invalid request body",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				})
				return
			}
		}
	}

	if req.Days <= 0 {
		req.Days = 365
	}
	if req.Workers <= 0 {
		req.Workers = 1
	}
	if req.Delay <= 0 {
		req.Delay = 5
	}

	// Normalize asset names to uppercase for consistency.
	for i, a := range req.Assets {
		req.Assets[i] = strings.ToUpper(a)
	}

	if err := h.syncFunc(tiingoKey, databaseURL, req); err != nil {
		slog.Error("Sync failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status:    "error",
			Message:   "Sync failed: " + err.Error(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	writeJSON(w, http.StatusOK, model.SyncResponse{
		Status:    "ok",
		Message:   "Sync completed",
		Assets:    req.Assets,
		Days:      req.Days,
		Workers:   req.Workers,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// requireBearerAuth checks that the request has an Authorization: Bearer <token>
// header whose token matches the TECH_INDICATOR_API_KEY environment variable.
// Empty tokens and empty expected keys are always rejected.
func requireBearerAuth(r *http.Request) bool {
	expected := os.Getenv("TECH_INDICATOR_API_KEY")
	if expected == "" {
		return false
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return false
	}

	return token == expected
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
