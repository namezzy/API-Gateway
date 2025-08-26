package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"api-gateway/internal/config"
)

var Logger *logrus.Logger

// Init 初始化日志系统
func Init(cfg config.LoggingConfig) error {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Logger.SetLevel(level)

	// 设置日志格式
	switch cfg.Format {
	case "json":
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	default:
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置输出
	if cfg.Output != "" && cfg.Output != "stdout" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.Output)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		Logger.SetOutput(io.MultiWriter(os.Stdout, file))
	} else {
		Logger.SetOutput(os.Stdout)
	}

	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		Logger = logrus.New()
	}
	return Logger
}

// WithFields 创建带字段的日志条目
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// WithField 创建带单个字段的日志条目
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// Info 记录信息级别日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 记录格式化信息级别日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Error 记录错误级别日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 记录格式化错误级别日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Warn 记录警告级别日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 记录格式化警告级别日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Debug 记录调试级别日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 记录格式化调试级别日志
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Fatal 记录致命错误级别日志并退出
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 记录格式化致命错误级别日志并退出
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}
