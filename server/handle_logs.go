package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/voilelab/plainshelf/internal/logutil"
)

// logHandlers lists and serves the log files the configured loggers write.
// It walks AppConf itself, so it holds the config rather than a copy of the
// paths, which would go stale.
type logHandlers struct {
	*apiCore

	conf *AppConf
}

type LogFileEntry = logutil.Entry

// GET /api/logs
func (h *logHandlers) getLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := logutil.ListLogFilesForSources(h.logSources())
	if err != nil {
		h.Error("failed to list log files", "error", err)
		http.Error(w, "failed to list log files", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, logs)
}

// GET /api/logs/{log_id}/content?tail_bytes=N
//
// The response is the end of the file rather than all of it: a log file that
// has been written to for weeks does not fit in the page that displays it. The
// caller pairs this with the size reported by GET /api/logs to tell whether it
// holds the whole file.
func (h *logHandlers) getLogContent(w http.ResponseWriter, r *http.Request) {
	logID, err := readLogID(r)
	if err != nil {
		http.Error(w, "invalid log_id", http.StatusBadRequest)
		return
	}

	tailBytes, ok := parseLogTailBytes(r)
	if !ok {
		http.Error(w, "invalid tail_bytes", http.StatusBadRequest)
		return
	}

	entry, fp, err := logutil.OpenLogFileByID(h.logSources(), logID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "log file not found", http.StatusNotFound)
			return
		}
		h.Error("failed to open log file", "error", err, "log_id", logID)
		http.Error(w, "failed to open log file", http.StatusInternalServerError)
		return
	}
	defer fp.Close()

	if _, err := logutil.SeekTail(fp, tailBytes); err != nil {
		h.Error("failed to seek log file", "error", err, "log_id", logID)
		http.Error(w, "failed to read log file", http.StatusInternalServerError)
		return
	}

	h.streamTextFile(w, fp, "failed to write log file content", "log_id", logID, "filename", entry.Filename)
}

// parseLogTailBytes reads the tail_bytes query parameter: absent applies
// logutil.DefaultTailBytes, and an explicit 0 asks for the whole file, which is
// how a caller that means to download it says so.
func parseLogTailBytes(r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("tail_bytes"))
	if raw == "" {
		return logutil.DefaultTailBytes, true
	}
	tailBytes, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || tailBytes < 0 {
		return 0, false
	}
	return tailBytes, true
}

func (h *logHandlers) logSources() []logutil.SourceConf {
	sources := make([]logutil.SourceConf, 0)
	collectLogSources(reflect.ValueOf(h.conf), "", &sources)
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
	if v.Type() == reflect.TypeFor[logutil.LogConf]() {
		logConf := v.Interface().(logutil.LogConf)
		*sources = append(*sources, logutil.SourceConf{
			Name:    prefix,
			LogFile: logConf.LogFile,
		})
		return
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			collectLogSources(v.Index(i), indexedLogSource(prefix, i), sources)
		}
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

func indexedLogSource(prefix string, index int) string {
	return fmt.Sprintf("%s[%d]", prefix, index)
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
