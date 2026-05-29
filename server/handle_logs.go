package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

type LogFileEntry struct {
	Filename string `json:"filename"`
	Date     string `json:"date"`
}

// GET /api/logs
func (app *App) HandleAPIGetLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := listLogFiles(app.conf.Logger.LogFile)
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

func listLogFiles(conf logutil.LogFileConf) ([]LogFileEntry, error) {
	switch conf.Type {
	case logutil.LogFileTypeNameRotate:
		return listRotatedLogFiles(conf)
	case logutil.LogFileTypeName:
		return listNamedLogFile(conf)
	default:
		return []LogFileEntry{}, nil
	}
}

func listRotatedLogFiles(conf logutil.LogFileConf) ([]LogFileEntry, error) {
	dir := conf.Dir
	if dir == "" {
		dir = "."
	}
	prefix := conf.Prefix
	if prefix == "" {
		prefix = "log"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogFileEntry{}, nil
		}
		return nil, util.Errorf("%w", err)
	}

	logs := make([]LogFileEntry, 0, len(entries))
	prefixPart := prefix + "-"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, prefixPart) || !strings.HasSuffix(name, ".log") {
			continue
		}

		date := strings.TrimSuffix(strings.TrimPrefix(name, prefixPart), ".log")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}

		logs = append(logs, LogFileEntry{
			Filename: name,
			Date:     date,
		})
	}

	sort.Slice(logs, func(i, j int) bool {
		if logs[i].Date == logs[j].Date {
			return logs[i].Filename < logs[j].Filename
		}
		return logs[i].Date > logs[j].Date
	})

	return logs, nil
}

func listNamedLogFile(conf logutil.LogFileConf) ([]LogFileEntry, error) {
	if conf.Filename == "" {
		return []LogFileEntry{}, nil
	}

	info, err := os.Stat(conf.Filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogFileEntry{}, nil
		}
		return nil, util.Errorf("%w", err)
	}
	if info.IsDir() {
		return []LogFileEntry{}, nil
	}

	return []LogFileEntry{{
		Filename: filepath.Base(conf.Filename),
		Date:     info.ModTime().Format("2006-01-02"),
	}}, nil
}
