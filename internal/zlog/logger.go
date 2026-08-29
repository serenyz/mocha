package zlog

import (
	"mmchat/internal/config"
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	cfg := config.GetConfig().LogConfig
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.CallerKey = "caller"

	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 控制台输出
	consoleWriter := zapcore.AddSync(os.Stdout)

	// 文件输出 + 日志轮转
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSize,    // 单个文件最大 100MB
		MaxBackups: cfg.MaxBackups, // 最多保留 30 个旧文件
		MaxAge:     cfg.MaxAge,     // 最多保留 7 天
		Compress:   true,
	})

	level := parseLevel(cfg.Level)

	// 同时写 stdout 和文件
	core := zapcore.NewTee(
		zapcore.NewCore(
			encoder,
			consoleWriter,
			level,
		),
		zapcore.NewCore(
			encoder,
			fileWriter,
			level,
		),
	)

	logger = zap.New(
		core,

		// 自动记录调用位置
		zap.AddCaller(),

		// 因为外面包了一层 zlog.Info，所以跳过一层
		zap.AddCallerSkip(1),

		// Error 及以上打印调用栈
		zap.AddStacktrace(zap.ErrorLevel),
	)
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}
