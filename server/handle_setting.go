package server

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (app *App) coverToJPG() bool {
	val := app.conf.CoverToJPG

	bs, exists, err := app.storeDB.GetSetting("cover_to_jpg")
	if err != nil {
		app.Logger.Error("coverToJPG:", "err", err)
	} else if exists {
		val = string(bs) == "true"
	}

	return val
}

func (app *App) readHistoryLimit() int {
	val := app.conf.ReadHistoryLimit

	bs, exists, err := app.storeDB.GetSetting("read_history_limit")
	if err != nil {
		app.Logger.Error("readHistoryLimit:", "err", err)
	} else if exists {
		parsed, err := strconv.Atoi(strings.TrimSpace(string(bs)))
		if err != nil || parsed < 0 {
			app.Logger.Error("readHistoryLimit: invalid stored value", "value", string(bs), "err", err)
		} else {
			val = parsed
		}
	}

	if val < 0 {
		return 0
	}
	return val
}

// GET /api/setting/cover_to_jpg
func (app *App) HandleGetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	val := app.coverToJPG()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if val {
		w.Write([]byte(`{"value": true}`))
	} else {
		w.Write([]byte(`{"value": false}`))
	}
}

// POST /api/setting/cover_to_jpg
func (app *App) HandleSetSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Logger.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if string(bs) != "true" && string(bs) != "false" {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting("cover_to_jpg", bs); err != nil {
		app.Logger.Error("SetSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/cover_to_jpg
func (app *App) HandleDeleteSettingCoverToJPG(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("cover_to_jpg"); err != nil {
		app.Logger.Error("DeleteSettingCoverToJPG:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/setting/read_history_limit
func (app *App) HandleGetSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	val := app.readHistoryLimit()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(fmt.Appendf(nil, `{"value": %d}`, val))
}

// POST /api/setting/read_history_limit
func (app *App) HandleSetSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		app.Logger.Error("read request body:", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	val, err := strconv.Atoi(strings.TrimSpace(string(bs)))
	if err != nil || val < 0 {
		http.Error(w, fmt.Sprintf("invalid value: %q", bs), http.StatusBadRequest)
		return
	}

	if err := app.storeDB.SetSetting("read_history_limit", []byte(strconv.Itoa(val))); err != nil {
		app.Logger.Error("SetSettingReadHistoryLimit:", "err", err)
		http.Error(w, "failed to save setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/setting/read_history_limit
func (app *App) HandleDeleteSettingReadHistoryLimit(w http.ResponseWriter, r *http.Request) {
	if err := app.storeDB.DeleteSetting("read_history_limit"); err != nil {
		app.Logger.Error("DeleteSettingReadHistoryLimit:", "err", err)
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
