package logutil

import (
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

type Entry struct {
	Filename string `json:"filename"`
	Date     string `json:"date"`
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
	switch conf.Type {
	case LogFileTypeNameRotate:
		return listRotatedLogFiles(conf)
	case LogFileTypeName:
		return listNamedLogFile(conf)
	default:
		return []Entry{}, nil
	}
}

func listRotatedLogFiles(conf LogFileConf) ([]Entry, error) {
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

		logs = append(logs, Entry{
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

func listNamedLogFile(conf LogFileConf) ([]Entry, error) {
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

	return []Entry{{
		Filename: filepath.Base(conf.Filename),
		Date:     info.ModTime().Format("2006-01-02"),
	}}, nil
}
