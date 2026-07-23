package utils

import (
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// UseJetStoreLogger sets up a zap logger and redirects the standard library log output to it.
func UseJetStoreLogger() {
	// For some users, the presets offered by the NewProduction, NewDevelopment,
	// and NewExample constructors won't be appropriate. For most of those
	// users, the bundled Config struct offers the right balance of flexibility
	// and convenience. (For more complex needs, see the AdvancedConfiguration
	// example.)
	//
	// See the documentation for Config and zapcore.EncoderConfig for all the
	// available options.

	EncoderCfg := zap.NewProductionEncoderConfig()
	EncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      false,
		DisableCaller:    true,
		Encoding:         "json",
		EncoderConfig:    EncoderCfg,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger := zap.Must(cfg.Build())
	defer logger.Sync()
	// logger.Info("logger construction succeeded")

	// Replace the system logger with our new logger. This allows us to use the standard library
	zap.RedirectStdLog(logger)
	// undo := zap.RedirectStdLog(logger)
	// defer undo()

	log.Print("redirected standard library logging to zap logger")
}
