package logger

import "go.uber.org/zap"

type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}

type ZapLogger struct {
	logger *zap.Logger
}

func NewZapLogger(logLevel string) (Logger, error) {
	parsedLevel, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, err
	}

	config := zap.Config{
		Encoding:         "json",
		Level:            parsedLevel,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig:    zap.NewProductionEncoderConfig(),
	}
	logger, err := config.Build()
	if err != nil {
		return nil, err
	}
	defer logger.Sync()

	return &ZapLogger{logger: logger}, nil
}

func (l *ZapLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *ZapLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

func (l *ZapLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

func (l *ZapLogger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}
