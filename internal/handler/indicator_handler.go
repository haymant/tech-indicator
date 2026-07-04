package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"time"

	"vercel-go-starter/internal/indicator"
	"vercel-go-starter/internal/model"

	"github.com/cinar/indicator/v2/asset"
	"github.com/cinar/indicator/v2/helper"
)

func (h *Handler) handleListIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status: "error", Message: "Method not allowed", Timestamp: now(),
		})
		return
	}

	entries := make([]model.IndicatorEntry, 0, len(indicator.Registry))
	catCounts := make(map[string]int)
	catDescs := map[string]string{
		"trend":      "Identify the direction and strength of price trends",
		"momentum":   "Measure the speed and magnitude of price movements",
		"volatility": "Measure the rate and magnitude of price fluctuations",
		"volume":     "Analyze trading volume to confirm price movements",
	}

	keys := make([]string, 0, len(indicator.Registry))
	for k := range indicator.Registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		def := indicator.Registry[k]
		entries = append(entries, model.IndicatorEntry{
			Name:          def.Key,
			Category:      def.Category,
			DisplayName:   def.DisplayName,
			Description:   def.Description,
			WhenToUse:     def.WhenToUse,
			Inputs:        def.Inputs,
			Outputs:       def.Outputs,
			SubIndicators: def.SubIndicators,
			DefaultParams: def.DefaultParams,
		})
		catCounts[def.Category]++
	}

	categories := make(map[string]model.CatalogCategory)
	for cat, desc := range catDescs {
		categories[cat] = model.CatalogCategory{
			Count:       catCounts[cat],
			Description: desc,
		}
	}

	writeJSON(w, http.StatusOK, model.IndicatorCatalogResponse{
		Indicators: entries,
		Count:      len(entries),
		Categories: categories,
		Timestamp:  now(),
	})
}

func (h *Handler) handleCalculateIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{
			Status: "error", Message: "Method not allowed", Timestamp: now(),
		})
		return
	}

	if !requireBearerAuth(r) {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{
			Status: "error", Message: "Unauthorized", Timestamp: now(),
		})
		return
	}

	var req model.IndicatorCalculateRequest
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err == nil && len(data) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Status: "error", Message: "Invalid request body", Timestamp: now(),
				})
				return
			}
		}
	}

	// Determine indicators to compute.
	if len(req.Indicators) == 0 {
		req.Indicators = allIndicatorKeys()
	} else {
		for _, name := range req.Indicators {
			if _, ok := indicator.Registry[name]; !ok {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Status: "error", Message: "Unknown indicator: " + name, Timestamp: now(),
				})
				return
			}
		}
	}

	if req.Days <= 0 {
		req.Days = 365
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "DATABASE_URL not set", Timestamp: now(),
		})
		return
	}

	// Ensure indicators table exists.
	if err := indicator.EnsureIndicatorsTable(databaseURL); err != nil {
		slog.Error("Failed to create indicators table", "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Status: "error", Message: "Database setup failed", Timestamp: now(),
		})
		return
	}

	// Determine assets.
	if len(req.Assets) == 0 {
		// Get all assets from snapshots table.
		repo, err := asset.NewRepository("motherduck", databaseURL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Status: "error", Message: "Failed to connect to database", Timestamp: now(),
			})
			return
		}
		assets, err := repo.Assets()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Status: "error", Message: "Failed to list assets", Timestamp: now(),
			})
			return
		}
		req.Assets = assets
	}

	processedAssets := 0

	for _, assetName := range req.Assets {
		if err := calculateForAsset(databaseURL, assetName, req.Indicators, req.Days); err != nil {
			slog.Error("Calculation failed for asset", "asset", assetName, "error", err)
			continue
		}
		processedAssets++
	}

	writeJSON(w, http.StatusOK, model.IndicatorCalculateResponse{
		Status:     "ok",
		Message:    "Indicator calculation completed",
		Assets:     req.Assets,
		Indicators: len(req.Indicators),
		AssetCount: processedAssets,
		Timestamp:  now(),
	})
}

func calculateForAsset(databaseURL, assetName string, indicatorKeys []string, days int) error {
	ctx := context.Background()

	repo, err := asset.NewRepository("motherduck", databaseURL)
	if err != nil {
		return err
	}

	startDate := time.Now().AddDate(0, 0, -days)
	snapshots, err := repo.GetSince(assetName, startDate)
	if err != nil {
		return err
	}

	snapshotSlice := helper.ChanToSlice(snapshots)
	if len(snapshotSlice) == 0 {
		slog.Warn("No snapshots found", "asset", assetName)
		return nil
	}

	// Extract dates for alignment.
	dates := make([]time.Time, len(snapshotSlice))
	for i, s := range snapshotSlice {
		dates[i] = s.Date
	}

	results := indicator.ComputeIndicators(ctx, snapshotSlice, indicatorKeys)
	if len(results) == 0 {
		return nil
	}

	return indicator.BatchUpsertIndicators(databaseURL, assetName, dates, results)
}

func allIndicatorKeys() []string {
	keys := make([]string, 0, len(indicator.Registry))
	for k := range indicator.Registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
