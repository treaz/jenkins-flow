package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Log levels
type Level int

const (
	Error Level = iota
	Info
	Debug
	Trace
)

func (l Level) String() string {
	switch l {
	case Error:
		return "ERROR"
	case Info:
		return "INFO"
	case Debug:
		return "DEBUG"
	case Trace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "ERROR":
		return Error, nil
	case "INFO":
		return Info, nil
	case "DEBUG":
		return Debug, nil
	case "TRACE":
		return Trace, nil
	default:
		return Info, fmt.Errorf("unknown log level: %s", s)
	}
}

// Logger is a thread-safe leveled logger
type Logger struct {
	mu     sync.RWMutex
	level  Level
	stdLog *log.Logger
}

// New creates a new Logger
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		stdLog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile),
	}
}

// SetLevel changes the log level safely
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Output writes the log if the level is sufficient
func (l *Logger) output(level Level, format string, args ...interface{}) {
	if l.GetLevel() >= level {
		prefix := fmt.Sprintf("[%s] ", level.String())
		// We use Output(2, ...) to skip this function and the wrapper
		l.stdLog.SetPrefix(prefix)
		l.stdLog.Output(3, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.output(Error, format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.output(Info, format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.output(Debug, format, args...)
}

func (l *Logger) Tracef(format string, args ...interface{}) {
	l.output(Trace, format, args...)
}

// SetOutput allows changing the output destination (stdLog is private but we can expose this if needed)
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stdLog.SetOutput(w)
}
