package logger

import (
	"fmt"
	"os"
	"time"

	"Qavor/pkg/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log            *zap.Logger
	sugar          *zap.SugaredLogger
	fileLog        *zap.Logger
	httpConsoleLog *zap.Logger
)

type httpRequestDetails struct {
	Status int
	Method string
	Path   string
	Query  string
}

// Init 初始化日志系统
func Init(cfg *config.LogConfig) error {
	encoderConfig := newEncoderConfig()

	// 日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// 文件输出
	fileWriter := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(fileWriter),
		level,
	)
	consoleCore := zapcore.NewCore(
		newConsoleEncoder(),
		zapcore.AddSync(os.Stdout),
		level,
	)

	loggerOptions := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel)}
	log = zap.New(zapcore.NewTee(fileCore, consoleCore), loggerOptions...)
	fileLog = zap.New(fileCore, loggerOptions...)
	httpConsoleLog = zap.New(consoleCore, loggerOptions...)
	sugar = log.Sugar()

	return nil
}

func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func newConsoleEncoder() zapcore.Encoder {
	consoleConfig := newEncoderConfig()
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	return zapcore.NewConsoleEncoder(consoleConfig)
}

// HTTPRequest records structured request data to the log file and a compact, colorized line to the console.
func HTTPRequest(method, path, query string, status int, latency time.Duration, ip, userAgent, requestErrors string) {
	details := httpRequestDetails{Status: status, Method: method, Path: path, Query: query}
	fileFields := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.String("query", query),
		zap.Int("status", status),
		zap.String("ip", ip),
		zap.String("user_agent", userAgent),
		zap.Duration("latency", latency),
	}
	if requestErrors != "" {
		fileFields = append(fileFields, zap.String("errors", requestErrors))
	}
	fileLog.Info("HTTP Request", fileFields...)

	consoleFields := []zap.Field{
		zap.Duration("latency", latency),
		zap.String("ip", ip),
	}
	if requestErrors != "" {
		consoleFields = append(consoleFields, zap.String("errors", requestErrors))
	}
	httpConsoleLog.Info(formatHTTPRequestConsoleMessage(details), consoleFields...)
}

func formatHTTPRequestConsoleMessage(details httpRequestDetails) string {
	requestPath := details.Path
	if details.Query != "" {
		requestPath += "?" + details.Query
	}
	return fmt.Sprintf("%s %s %s", colorStatus(details.Status), details.Method, requestPath)
}

func colorStatus(status int) string {
	const reset = "\x1b[0m"
	color := ""
	switch {
	case status >= 200 && status < 300:
		color = "\x1b[32m"
	case status >= 300 && status < 400:
		color = "\x1b[36m"
	case status >= 400 && status < 500:
		color = "\x1b[33m"
	case status >= 500:
		color = "\x1b[31m"
	}
	if color == "" {
		return fmt.Sprintf("%d", status)
	}
	return color + fmt.Sprintf("%d", status) + reset
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	log.Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	log.Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	log.Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

// Fatal 致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	log.Fatal(msg, fields...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	sugar.Debugf(format, args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	sugar.Infof(format, args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	sugar.Warnf(format, args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	sugar.Fatalf(format, args...)
}

// With 创建带字段的 logger
func With(fields ...zap.Field) *zap.Logger {
	return log.With(fields...)
}

// Sync 同步日志缓冲区
func Sync() error {
	return log.Sync()
}

// GetLogger 获取原始 logger
func GetLogger() *zap.Logger {
	return log
}

// GetSugaredLogger 获取 sugared logger
func GetSugaredLogger() *zap.SugaredLogger {
	return sugar
}
