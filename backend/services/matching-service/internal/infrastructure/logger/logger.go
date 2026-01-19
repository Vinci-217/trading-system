package main

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
	if !l.shouldLog(level) {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	location := l.getCallerLocation()

	formattedMsg := l.formatMessage(msg, args...)
	log.Printf("[%s] [%s] [%s] %s\n", timestamp, level, location, formattedMsg)
}

func (l *Logger) shouldLog(level string) bool {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	currentLevel := levels[l.level]
	targetLevel := levels[level]

	return targetLevel >= currentLevel
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
		key := fmt.Sprint(args[i])
		value := fmt.Sprint(args[i+1])
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(value)
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

type LogField struct {
	key   string
	value interface{}
}

func String(key string, value string) LogField {
	return LogField{key, value}
}

func Int(key string, value int) LogField {
	return LogField{key, value}
}

func Int64(key string, value int64) LogField {
	return LogField{key, value}
}

func Float64(key string, value float64) LogField {
	return LogField{key, value}
}

func Bool(key string, value bool) LogField {
	return LogField{key, value}
}

func Duration(key string, value time.Duration) LogField {
	return LogField{key, value}
}

func Error(key string, value error) LogField {
	return LogField{key, value}
}

func (l *Logger) String(key string, value string) LogField {
	return String(key, value)
}

func (l *Logger) Int(key string, value int) LogField {
	return Int(key, value)
}

func (l *Logger) Int64(key string, value int64) LogField {
	return Int64(key, value)
}

func (l *Logger) Float64(key string, value float64) LogField {
	return Float64(key, value)
}

func (l *Logger) Bool(key string, value bool) LogField {
	return Bool(key, value)
}

func (l *Logger) Duration(key string, value time.Duration) LogField {
	return Duration(key, value)
}

func (l *Logger) ErrorField(key string, value error) LogField {
	return Error(key, value)
}
