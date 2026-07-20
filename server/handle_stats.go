package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultReadingActivityRangeDays = 365

type readingActivityRequest struct {
	BookID  string `json:"book_id"`
	Seconds int    `json:"seconds"`
	Date    string `json:"date"`
}

type readingActivityDay struct {
	TotalSeconds int `json:"total_seconds"`
}

type readingActivityResponse struct {
	Days map[string]readingActivityDay `json:"days"`
	Unit string                        `json:"unit"`
}

// POST /api/shelves/{shelf_id}/reading_activity
//
// Records reading time for a book. date must be the server's local today or
// yesterday (to tolerate clients that heartbeat across midnight); any other
// date is clamped to today rather than rejected, since a client's local clock
// may legitimately disagree with the server's by a day.
func (app *App) HandleAPIPostReadingActivity(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	var req readingActivityRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	bookID := strings.TrimSpace(req.BookID)
	if bookID == "" {
		http.Error(w, "missing book_id", http.StatusBadRequest)
		return
	}

	dateStr := strings.TrimSpace(req.Date)
	var date time.Time
	if dateStr != "" {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	now := time.Now()
	today := now
	yesterday := now.AddDate(0, 0, -1)
	if date.IsZero() || (date != today && date != yesterday) {
		// Clamp out-of-window dates to today rather than rejecting outright.
		date = today
	}

	if err := shelfData.ReadingStats().AddSeconds(date, bookID, req.Seconds); err != nil {
		app.Error("failed to record reading activity", "error", err)
		http.Error(w, "failed to record reading activity", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/shelves/{shelf_id}/reading_activity?from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Returns per-day total reading seconds only - no per-book breakdown is
// exposed here. from/to default to the past defaultReadingActivityRangeDays
// days when omitted.
func (app *App) HandleAPIGetReadingActivity(w http.ResponseWriter, r *http.Request) {
	shelfID, err := readShelfID(r)
	if err != nil {
		http.Error(w, "invalid shelf_id", http.StatusBadRequest)
		return
	}

	shelfData, ok := app.shelfManager.GetShelf(shelfID)
	if !ok {
		http.Error(w, "shelf not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	from := now.AddDate(0, 0, -defaultReadingActivityRangeDays)
	to := now
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		var err error
		from, err = time.Parse("2006-01-02", v)
		if err != nil {
			http.Error(w, "invalid from date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		var err error
		to, err = time.Parse("2006-01-02", v)
		if err != nil {
			http.Error(w, "invalid to date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	rangeResult, err := shelfData.ReadingStats().Range(from, to)
	if err != nil {
		http.Error(w, "invalid from/to range", http.StatusBadRequest)
		return
	}

	days := make(map[string]readingActivityDay, len(rangeResult))
	for date, day := range rangeResult {
		days[date] = readingActivityDay{TotalSeconds: day.TotalSeconds}
	}

	resp := readingActivityResponse{
		Days: days,
		Unit: "seconds",
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
