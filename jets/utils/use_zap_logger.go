package utils

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// jetsLogger is the logger installed by UseJetStoreLogger, nil when not installed (e.g. dev mode)
var jetsLogger *zap.Logger

// UseJetStoreLogger sets up a zap logger and redirects the standard library log output to it.
func UseJetStoreLogger() {
	// Don't use the zap logger if running in dev mode locally
	_, globalDevMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if globalDevMode {
		log.Print("JETSTORE_DEV_MODE is set, using standard library logger instead of zap logger")
		return
	}

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
	jetsLogger = logger
	// logger.Info("logger construction succeeded")

	// Replace the system logger with our new logger. This allows us to use the standard library
	zap.RedirectStdLog(logger)
	// undo := zap.RedirectStdLog(logger)
	// defer undo()

	log.Print("redirected standard library logging to zap logger")
}

// LogFatal logs msg and err at error level with the structured logger and exits with status 1.
// Use it in place of log.Fatal or log.Panic: log.Panic logs at info level and the panic then
// writes the raw message and a goroutine dump to stderr, bypassing the structured logger.
func LogFatal(msg string, err error) {
	if jetsLogger == nil {
		log.Printf("%s: %v", msg, err)
		os.Exit(1)
	}
	jetsLogger.Error(msg, zap.Error(err))
	jetsLogger.Sync()
	os.Exit(1)
}

// LogPanicAndExit logs a panic with its stack trace using LogFatal.
// Defer it at the top of main: defer utils.LogPanicAndExit()
func LogPanicAndExit() {
	if r := recover(); r != nil {
		LogFatal("panic", fmt.Errorf("%v\n%s", r, debug.Stack()))
	}
}
