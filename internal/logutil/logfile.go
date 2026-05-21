package logutil

import (
	"io"
	"os"

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
	if lf.fp != nil {
		err := lf.fp.Close()
		if err != nil {
			return util.Errorf("%w", err)
		}
	}
	return nil
}
