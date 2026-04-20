package logger

import "go.uber.org/zap"

// New returns a zap.Logger appropriate for the given environment.
func New(env string) *zap.Logger {
	var (
		l   *zap.Logger
		err error
	)
	if env == "development" {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}
	if err != nil {
		return zap.NewNop()
	}
	return l
}
