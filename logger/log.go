package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger(namespace, environment, logLevel string) {
	var zapLogLevel zapcore.Level = zap.InfoLevel
	if logLevel == "debug" {
		zapLogLevel = zap.DebugLevel
	}

	zapConfig := zap.NewProductionConfig()

	zapConfig.Level.SetLevel(zapLogLevel)
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, _ := zapConfig.Build()
	logger = logger.WithOptions(zap.AddStacktrace(zapcore.FatalLevel)).With(zap.String("namespace", namespace)).With(zap.String("environment", environment))
	zap.ReplaceGlobals(logger)
}
