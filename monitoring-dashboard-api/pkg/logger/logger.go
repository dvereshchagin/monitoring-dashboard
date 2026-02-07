package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	logger *log.Logger
	level  Level
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
		logger: log.New(os.Stdout, "", 0),
		level:  parseLevel(level),
	}
	return l
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
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("[%s] [%s] %s", timestamp, level, msg)

	if len(args) > 0 {
		message += " |"
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				message += fmt.Sprintf(" %v=%v", args[i], args[i+1])
			}
		}
	}

	l.logger.Println(message)
}
