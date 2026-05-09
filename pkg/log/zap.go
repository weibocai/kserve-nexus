package log

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

var Logger logr.Logger

func NewDefaultZapLogger() (logr.Logger, error) {
	// change the configuration in the future if needed.
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return logr.Discard(), err
	}
	logger := zapr.NewLogger(zapLogger)
	return logger, nil
}

func init() {
	l, err := NewDefaultZapLogger()
	if err != nil {
		panic(err)
	}
	Logger = l
}
