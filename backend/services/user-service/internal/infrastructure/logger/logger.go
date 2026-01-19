package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

type Logger struct {
	level string
}

func NewLogger(level string) *Logger {
	return &Logger{level: level}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log("INFO", msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log("ERROR", msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log("WARN", msg, args...)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level == "debug" {
		l.log("DEBUG", msg, args...)
	}
}

func (l *Logger) log(level, msg string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	location := l.getCallerLocation()
	formattedMsg := l.formatMessage(msg, args...)
	log.Printf("[%s] [%s] [%s] %s\n", timestamp, level, location, formattedMsg)
}

func (l *Logger) formatMessage(msg string, args ...interface{}) string {
	if len(args) == 0 {
		return msg
	}

	var sb strings.Builder
	sb.WriteString(msg)

	if len(args)%2 == 1 {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprint(args[0]))
		args = args[1:]
	}

	for i := 0; i < len(args); i += 2 {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprint(args[i]))
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(args[i+1]))
	}

	return sb.String()
}

func (l *Logger) getCallerLocation() string {
	pc, _, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown:0"
	}

	funcName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(funcName, "/")
	funcName = parts[len(parts)-1]

	return fmt.Sprintf("%s:%d", funcName, line)
}

func String(key string, value string) interface{}   { return fmt.Sprintf("%s=%s", key, value) }
func Int(key string, value int) interface{}         { return fmt.Sprintf("%s=%d", key, value) }
func Int64(key string, value int64) interface{}     { return fmt.Sprintf("%s=%d", key, value) }
func Float64(key string, value float64) interface{} { return fmt.Sprintf("%s=%.4f", key, value) }
func Bool(key string, value bool) interface{}       { return fmt.Sprintf("%s=%t", key, value) }
func Duration(key string, value time.Duration) interface{} { return fmt.Sprintf("%s=%v", key, value) }
func Error(key string, value error) interface{}     { return fmt.Sprintf("%s=%v", key, value) }
