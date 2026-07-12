// Package logger provides a structured logger for NASIJ built on zap.
//
// Two modes are supported:
//   - "pretty": human-readable coloured console output (default for interactive use)
//   - "json":   machine-readable JSON output (suitable for CI and log aggregators)
package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a configured *zap.Logger.
//
//   - level:  one of "debug", "info", "warn", "error"
//   - format: one of "pretty", "json"
//
// The logger writes to stderr so it does not pollute structured stdout output.
func New(level, format string) (*zap.Logger, error) {
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("logger: invalid level %q: %w", level, err)
	}

	enc, err := buildEncoder(format)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(
		enc,
		zapcore.AddSync(os.Stderr),
		zap.NewAtomicLevelAt(zapLevel),
	)

	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0)), nil
}

// Nop returns a no-op logger that discards all output.
// Useful in tests that do not care about log output.
func Nop() *zap.Logger {
	return zap.NewNop()
}

// buildEncoder returns the zapcore.Encoder for the requested format.
func buildEncoder(format string) (zapcore.Encoder, error) {
	switch format {
	case "json":
		cfg := zap.NewProductionEncoderConfig()
		cfg.TimeKey = "ts"
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		return zapcore.NewJSONEncoder(cfg), nil

	case "pretty":
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncodeCaller = zapcore.ShortCallerEncoder
		cfg.ConsoleSeparator = "  "
		return zapcore.NewConsoleEncoder(cfg), nil

	default:
		return nil, fmt.Errorf("logger: invalid format %q (must be one of: json, pretty)", format)
	}
}
