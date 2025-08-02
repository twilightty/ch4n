package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel represents different log levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns string representation of log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger represents the application logger
type Logger struct {
	level    LogLevel
	logger   *log.Logger
	file     *os.File
	filename string
}

// NewLogger creates a new logger instance
func NewLogger(level string, filename string) (*Logger, error) {
	logLevel := parseLogLevel(level)
	
	var writers []io.Writer
	
	// Always write to stdout
	writers = append(writers, os.Stdout)
	
	var file *os.File
	var err error
	
	// If filename provided, also write to file
	if filename != "" {
		// Create directory if it doesn't exist
		dir := filepath.Dir(filename)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %v", err)
			}
		}
		
		file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}
		writers = append(writers, file)
	}
	
	multiWriter := io.MultiWriter(writers...)
	logger := log.New(multiWriter, "", 0) // No default prefix, we'll add our own
	
	return &Logger{
		level:    logLevel,
		logger:   logger,
		file:     file,
		filename: filename,
	}, nil
}

// parseLogLevel converts string to LogLevel
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// log writes a log message with the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	prefix := fmt.Sprintf("[%s] [%s] ", timestamp, level.String())
	message := fmt.Sprintf(format, args...)
	
	l.logger.Printf("%s%s", prefix, message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

// Close closes the log file if it was opened
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level string) {
	l.level = parseLogLevel(level)
}

// GetLevel returns current log level as string
func (l *Logger) GetLevel() string {
	return l.level.String()
}
