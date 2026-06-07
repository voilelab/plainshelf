package server

import (
	"fmt"
	"io"
	"net/http"
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
