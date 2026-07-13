package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/shelf"
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

	date := strings.TrimSpace(req.Date)
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			http.Error(w, "invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	if date == "" || (date != today && date != yesterday) {
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
	from := now.AddDate(0, 0, -defaultReadingActivityRangeDays).Format("2006-01-02")
	to := now.Format("2006-01-02")
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		from = v
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		to = v
	}

	rangeResult, err := shelfData.ReadingStats().Range(from, to)
	if err != nil {
		if errors.Is(err, shelf.ErrInvalidDate) {
			http.Error(w, "invalid from/to date, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}
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
