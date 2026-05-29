package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
)

type LogFileEntry = logutil.Entry

// GET /api/logs
func (app *App) HandleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := logutil.ListLogFilesForSources(app.logSources())
	if err != nil {
		app.Error("failed to list log files", "error", err)
		http.Error(w, "failed to list log files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(logs)
	if err != nil {
		app.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GET /api/logs/{log_id}/content
func (app *App) HandleAPIGetLogContent(w http.ResponseWriter, r *http.Request) {
	logID, err := readLogID(r)
	if err != nil {
		http.Error(w, "invalid log_id", http.StatusBadRequest)
		return
	}

	entry, fp, err := logutil.OpenLogFileByID(app.logSources(), logID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "log file not found", http.StatusNotFound)
			return
		}
		app.Error("failed to open log file", "error", err, "log_id", logID)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer fp.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = io.Copy(w, fp)
	if err != nil {
		app.Error("failed to write log file content", "error", err, "log_id", logID, "filename", entry.Filename)
		http.Error(w, "failed to write log file content", http.StatusInternalServerError)
		return
	}
}

func (app *App) logSources() []logutil.SourceConf {
	sources := make([]logutil.SourceConf, 0)
	collectLogSources(reflect.ValueOf(app.conf), "", &sources)
	return sources
}

func collectLogSources(v reflect.Value, prefix string, sources *[]logutil.SourceConf) {
	if !v.IsValid() {
		return
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if !v.IsValid() {
		return
	}
	if v.Type() == reflect.TypeOf(logutil.LogConf{}) {
		logConf := v.Interface().(logutil.LogConf)
		*sources = append(*sources, logutil.SourceConf{
			Name:    prefix,
			LogFile: logConf.LogFile,
		})
		return
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		nextPrefix := joinLogSource(prefix, logSourceFieldName(field))
		collectLogSources(v.Field(i), nextPrefix, sources)
	}
}

func joinLogSource(prefix, name string) string {
	if name == "" {
		return prefix
	}
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func logSourceFieldName(field reflect.StructField) string {
	yamlTag := strings.TrimSpace(strings.Split(field.Tag.Get("yaml"), ",")[0])
	switch yamlTag {
	case "", "-":
		return strings.ToLower(field.Name)
	default:
		return yamlTag
	}
}
