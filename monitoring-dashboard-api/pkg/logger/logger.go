package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	applicationPort "github.com/dreschagin/monitoring-dashboard/internal/application/port"
)

type Logger struct {
	logger       *log.Logger
	level        Level
	logPublisher applicationPort.LogPublisher // Optional CloudWatch logs publisher
}

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func New(level string) *Logger {
	l := &Logger{
		logger:       log.New(os.Stdout, "", 0),
		level:        parseLevel(level),
		logPublisher: nil,
	}
	return l
}

// SetLogPublisher sets an optional log publisher for CloudWatch integration.
func (l *Logger) SetLogPublisher(publisher applicationPort.LogPublisher) {
	l.logPublisher = publisher
}

func parseLevel(level string) Level {
	switch level {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log("DEBUG", msg, args...)
	}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.log("INFO", msg, args...)
	}
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.log("WARN", msg, args...)
	}
}

func (l *Logger) Error(msg string, err error, args ...interface{}) {
	if l.level <= ERROR {
		if err != nil {
			args = append(args, "error", err.Error())
		}
		l.log("ERROR", msg, args...)
	}
}

func (l *Logger) log(level, msg string, args ...interface{}) {
	timestamp := time.Now()
	formattedTime := timestamp.Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("[%s] [%s] %s", formattedTime, level, msg)

	if len(args) > 0 {
		message += " |"
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				message += fmt.Sprintf(" %v=%v", args[i], args[i+1])
			}
		}
	}

	// Log to stdout
	l.logger.Println(message)

	// Publish to CloudWatch if publisher is configured
	if l.logPublisher != nil {
		entry := l.buildLogEntry(timestamp, level, msg, args...)
		// Use background context with timeout for CloudWatch publishing
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Don't fail the main operation if CloudWatch publish fails
		if err := l.logPublisher.Publish(ctx, entry); err != nil {
			// Log CloudWatch errors to stdout only (avoid infinite recursion)
			fmt.Fprintf(os.Stderr, "[WARN] Failed to publish log to CloudWatch: %v\n", err)
		}
	}
}

// buildLogEntry creates a structured LogEntry for CloudWatch.
func (l *Logger) buildLogEntry(timestamp time.Time, level, msg string, args ...interface{}) applicationPort.LogEntry {
	// Convert log level string to port.LogLevel
	var logLevel applicationPort.LogLevel
	switch level {
	case "DEBUG":
		logLevel = applicationPort.LogLevelDebug
	case "INFO":
		logLevel = applicationPort.LogLevelInfo
	case "WARN":
		logLevel = applicationPort.LogLevelWarn
	case "ERROR":
		logLevel = applicationPort.LogLevelError
	default:
		logLevel = applicationPort.LogLevelInfo
	}

	// Build fields map from args
	fields := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			fields[key] = args[i+1]
		}
	}

	return applicationPort.LogEntry{
		Timestamp: timestamp,
		Level:     logLevel,
		Message:   msg,
		Fields:    fields,
	}
}
