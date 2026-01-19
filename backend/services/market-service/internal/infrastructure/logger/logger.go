package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type Logger struct {
	name  string
	mu    sync.Mutex
	level string
}

func NewLogger(name string) *Logger {
	return &Logger{
		name:  name,
		level: "info",
	}
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.log("INFO", msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.log("WARN", msg, fields...)
}

func (l *Logger) Error(msg string, fields ...interface{}) {
	l.log("ERROR", msg, fields...)
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.log("DEBUG", msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.log("FATAL", msg, fields...)
	os.Exit(1)
}

func (l *Logger) log(level string, msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := make(map[string]interface{})
	entry["timestamp"] = time.Now().Format(time.RFC3339)
	entry["level"] = level
	entry["service"] = l.name
	entry["message"] = msg

	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		entry[key] = fields[i+1]
	}

	data, _ := json.Marshal(entry)
	fmt.Println(string(data))
}

func String(key string, value string) interface{} {
	return fmt.Sprintf("%s: %s", key, value)
}

func Int(key string, value int) interface{} {
	return fmt.Sprintf("%s: %d", key, value)
}

func Int64(key string, value int64) interface{} {
	return fmt.Sprintf("%s: %d", key, value)
}

func Float64(key string, value float64) interface{} {
	return fmt.Sprintf("%s: %f", key, value)
}

func Duration(key string, value time.Duration) interface{} {
	return fmt.Sprintf("%s: %v", key, value)
}

func Error(err error) interface{} {
	return fmt.Sprintf("error: %v", err)
}

func Decimal(key string, value decimal.Decimal) interface{} {
	return fmt.Sprintf("%s: %s", key, value.String())
}

func Bool(key string, value bool) interface{} {
	return fmt.Sprintf("%s: %t", key, value)
}
