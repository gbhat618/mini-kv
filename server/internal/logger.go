package internal

import (
	"log"
	"os"
	"strings"
)

var (
	logLevel  LogLevel
	logger    *log.Logger
	logPrefix string
)

func init() {
	logger = log.New(os.Stdout, "", 0)
}

func SetLogger(l *log.Logger) {
	logger = l
}

func SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		logLevel = DEBUG
	case "info":
		logLevel = INFO
	case "warn", "warning":
		logLevel = WARN
	case "error":
		logLevel = ERROR
	default:
		logLevel = INFO
	}
}

func GetLogLevel() LogLevel {
	return logLevel
}

func GetLogPrefix() string {
	return logPrefix
}

func SetLogPrefix(prefix string) {
	logPrefix = prefix
}

func Debug(format string, v ...interface{}) {
	if logLevel <= DEBUG {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	if logLevel <= INFO {
		logger.Printf("[INFO] "+format, v...)
	}
}

func Warn(format string, v ...interface{}) {
	if logLevel <= WARN {
		logger.Printf("[WARN] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if logLevel <= ERROR {
		logger.Printf("[ERROR] "+format, v...)
	}
}
