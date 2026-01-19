package logger

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger

func Init(serviceName string, level string, output string) error {
	zapLevel := parseLevel(level)
	
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	
	if output == "stdout" {
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	} else {
		logDir := output
		if !filepath.IsAbs(logDir) {
			logDir, _ = filepath.Abs(logDir)
		}
		os.MkdirAll(logDir, 0755)
		
		currentDate := time.Now().Format("2006-01-02")
		logPath := filepath.Join(logDir, serviceName+"-"+currentDate+".log")
		
		config.OutputPaths = []string{logPath}
		config.ErrorOutputPaths = []string{logPath}
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	logger, err := config.Build()
	if err != nil {
		return err
	}
	
	globalLogger = logger
	return nil
}

func GetLogger() *zap.Logger {
	if globalLogger == nil {
		// 返回一个默认logger
		logger, _ := zap.NewDevelopment()
		return logger
	}
	return globalLogger
}

func With(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

func Debugf(msg string, args ...interface{}) {
	GetLogger().Debugf(msg, args...)
}

func Infof(msg string, args ...interface{}) {
	GetLogger().Infof(msg, args...)
}

func Warnf(msg string, args ...interface{}) {
	GetLogger().Warnf(msg, args...)
}

func Errorf(msg string, args ...interface{}) {
	GetLogger().Errorf(msg, args...)
}

func Fatalf(msg string, args ...interface{}) {
	GetLogger().Fatalf(msg, args...)
}

func Sync() {
	if globalLogger != nil {
		globalLogger.Sync()
	}
}

func parseLevel(level string) zapcore.Level {
	level = strings.ToLower(level)
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

type Field = zap.Field

func Int(key string, value int) Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

func Float64(key string, value float64) Field {
	return zap.Float64(key, value)
}

func String(key string, value string) Field {
	return zap.String(key, value)
}

func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

func ErrorKey(err error) Field {
	return zap.Error(err)
}

func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}
