package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

type Logger struct {
	*zap.SugaredLogger
}

func NewLogger(level string) *Logger {
	once.Do(func() {
		var zapLevel zapcore.Level
		if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
			zapLevel = zapcore.InfoLevel
		}

		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
			zap.NewAtomicLevelAt(zapLevel),
		)

		globalLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	})

	return &Logger{SugaredLogger: globalLogger.Sugar()}
}

func (l *Logger) Sync() error {
	return globalLogger.Sync()
}

func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{SugaredLogger: l.SugaredLogger.With(fields...)}
}

func (l *Logger) String(key, val string) interface{} {
	return zap.String(key, val)
}

func (l *Logger) Int(key string, val int) interface{} {
	return zap.Int(key, val)
}

func (l *Logger) Int64(key string, val int64) interface{} {
	return zap.Int64(key, val)
}

func (l *Logger) Bool(key string, val bool) interface{} {
	return zap.Bool(key, val)
}

func (l *Logger) Error(err error) interface{} {
	return zap.Error(err)
}

func (l *Logger) Any(key string, val interface{}) interface{} {
	return zap.Any(key, val)
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.SugaredLogger.Infow(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.SugaredLogger.Warnw(msg, fields...)
}

func (l *Logger) Errorw(msg string, fields ...interface{}) {
	l.SugaredLogger.Errorw(msg, fields...)
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.SugaredLogger.Debugw(msg, fields...)
}
