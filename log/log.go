package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var s *zap.SugaredLogger

// default info level
var level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

func init() {
	config := zap.Config{
		Level:             level,
		Development:       true,
		DisableStacktrace: true,
		Encoding:          "console",
		EncoderConfig:     zap.NewDevelopmentEncoderConfig(),
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}

	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Errorf("failed to initialize logger: %v", err))
	}
	s = logger.Sugar()
}

func SetLevel(l zapcore.Level) {
	level.SetLevel(l)
}

func S() *zap.SugaredLogger {
	return s
}
