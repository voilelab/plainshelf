package logutil

import (
	"log/slog"

	"github.com/voilelab/plainshelf/internal/util"
)

type LogConf struct {
	Format string `yaml:"format"`

	Level string `yaml:"level"`

	LogFile LogFileConf `yaml:"log_file"`

	AddSource bool `yaml:"add_source"`
}

type Logger struct {
	*slog.Logger

	logFile *LogFile
}

func NewLogger(conf *LogConf) (*Logger, error) {
	var level slog.Level
	switch conf.Level {
	case "debug":
		level = slog.LevelDebug
	case "info", "":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, util.Errorf("invalid log level: %s", conf.Level)
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: conf.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				t := a.Value.Time()
				return slog.String(slog.TimeKey, t.Format("2006-01-02 15:04:05"))
			}
			return a
		},
	}

	logFile, err := NewLogFile(conf.LogFile)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	var h slog.Handler
	switch conf.Format {
	case "json", "":
		h = slog.NewJSONHandler(logFile.writer, opts)
	case "text":
		h = slog.NewTextHandler(logFile.writer, opts)
	default:
		return nil, util.Errorf("invalid log format: %s", conf.Format)
	}

	return &Logger{
		Logger:  slog.New(h),
		logFile: logFile,
	}, nil
}

func (l *Logger) With(args ...any) *Logger {
	if l == nil {
		return nil
	}
	return &Logger{Logger: l.Logger.With(args...)}
}

func (l *Logger) Close() error {
	if l.logFile == nil {
		return nil
	}

	err := l.logFile.Close()
	if err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
