package logutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/voilelab/plainshelf/internal/util"
)

type LogFileType string

const (
	LogFileTypeDefault    LogFileType = ""
	LogFileTypeStderr     LogFileType = "stderr"
	LogFileTypeStdout     LogFileType = "stdout"
	LogFileTypeNone       LogFileType = "none"
	LogFileTypeName       LogFileType = "filename"
	LogFileTypeNameRotate LogFileType = "filename_rotate"
)

type LogFileConf struct {
	// Type specifies the type of log file.
	// Valid values are "stderr", "stdout", "none", "filename", and "filename_rotate". Default is "stderr".
	Type LogFileType `yaml:"type"`

	// Filename is used when Type is "filename".
	Filename string `yaml:"filename"`

	// Dir and Prefix are used when Type is "filename_rotate".
	Dir    string `yaml:"dir"`
	Prefix string `yaml:"prefix"`
}

type LogFile struct {
	conf   *LogFileConf
	writer io.Writer
	fp     *os.File
}

type SourceConf struct {
	Name    string
	LogFile LogFileConf
}

type Entry struct {
	ID       string `json:"id"`
	Source   string `json:"source,omitempty"`
	Filename string `json:"filename"`
	Date     string `json:"date"`

	path string
}

func NewLogFile(conf LogFileConf) (*LogFile, error) {
	switch conf.Type {
	case LogFileTypeStderr, LogFileTypeDefault:
		return &LogFile{conf: &conf, writer: os.Stderr}, nil
	case LogFileTypeStdout:
		return &LogFile{conf: &conf, writer: os.Stdout}, nil
	case LogFileTypeNone:
		return &LogFile{conf: &conf, writer: io.Discard}, nil
	case LogFileTypeName:
		fp, err := os.OpenFile(conf.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		return &LogFile{conf: &conf, writer: fp, fp: fp}, nil
	case LogFileTypeNameRotate:
		writer := NewDailyFileWriter(conf.Dir, conf.Prefix)
		return &LogFile{conf: &conf, writer: writer}, nil
	default:
		return nil, util.Errorf("invalid log file type: %s", conf.Type)
	}
}

func (lf *LogFile) Close() error {
	switch lf.conf.Type {
	case LogFileTypeStderr, LogFileTypeStdout, LogFileTypeDefault, LogFileTypeNone:
		return nil
	}

	if closer, ok := lf.writer.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return util.Errorf("%w", err)
		}
	}
	return nil
}

func ListLogFiles(conf LogFileConf) ([]Entry, error) {
	return listLogFilesForSource("", conf)
}

func ListLogFilesForSources(confs []SourceConf) ([]Entry, error) {
	logs := make([]Entry, 0)
	seen := make(map[string]struct{}, len(confs))
	for _, conf := range confs {
		sourceLogs, err := listLogFilesForSource(conf.Name, conf.LogFile)
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		for _, entry := range sourceLogs {
			if _, ok := seen[entry.ID]; ok {
				continue
			}
			seen[entry.ID] = struct{}{}
			logs = append(logs, entry)
		}
	}
	sortEntries(logs)
	return logs, nil
}

func OpenLogFileByID(confs []SourceConf, id string) (Entry, *os.File, error) {
	logs, err := ListLogFilesForSources(confs)
	if err != nil {
		return Entry{}, nil, util.Errorf("%w", err)
	}
	for _, entry := range logs {
		if entry.ID != id {
			continue
		}
		fp, err := os.Open(entry.path)
		if err != nil {
			return Entry{}, nil, util.Errorf("%w", err)
		}
		return entry, fp, nil
	}
	return Entry{}, nil, os.ErrNotExist
}

func listLogFilesForSource(source string, conf LogFileConf) ([]Entry, error) {
	switch conf.Type {
	case LogFileTypeNameRotate:
		return listRotatedLogFiles(source, conf)
	case LogFileTypeName:
		return listNamedLogFile(source, conf)
	default:
		return []Entry{}, nil
	}
}

func listRotatedLogFiles(source string, conf LogFileConf) ([]Entry, error) {
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
			return []Entry{}, nil
		}
		return nil, util.Errorf("%w", err)
	}

	logs := make([]Entry, 0, len(entries))
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

		logs = append(logs, newEntry(source, name, date, filepath.Join(dir, name)))
	}

	sortEntries(logs)
	return logs, nil
}

func listNamedLogFile(source string, conf LogFileConf) ([]Entry, error) {
	if conf.Filename == "" {
		return []Entry{}, nil
	}

	info, err := os.Stat(conf.Filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, util.Errorf("%w", err)
	}
	if info.IsDir() {
		return []Entry{}, nil
	}

	return []Entry{newEntry(source, filepath.Base(conf.Filename), info.ModTime().Format("2006-01-02"), conf.Filename)}, nil
}

func newEntry(source, filename, date, path string) Entry {
	return Entry{
		ID:       makeEntryID(source, path),
		Source:   source,
		Filename: filename,
		Date:     date,
		path:     cleanLogPath(path),
	}
}

func makeEntryID(source, path string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + cleanLogPath(path)))
	return hex.EncodeToString(sum[:])
}

func cleanLogPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func sortEntries(logs []Entry) {
	sort.Slice(logs, func(i, j int) bool {
		if logs[i].Date != logs[j].Date {
			return logs[i].Date > logs[j].Date
		}
		if logs[i].Filename != logs[j].Filename {
			return logs[i].Filename < logs[j].Filename
		}
		if logs[i].Source != logs[j].Source {
			return logs[i].Source < logs[j].Source
		}
		return logs[i].ID < logs[j].ID
	})
}
