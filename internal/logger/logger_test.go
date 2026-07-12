package logger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/logger"
)

func TestNew_PrettyMode(t *testing.T) {
	log, err := logger.New("info", "pretty")
	require.NoError(t, err)
	require.NotNil(t, log)
	log.Info("test message from pretty logger")
}

func TestNew_JSONMode(t *testing.T) {
	log, err := logger.New("debug", "json")
	require.NoError(t, err)
	require.NotNil(t, log)
	log.Debug("test message from json logger")
}

func TestNew_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			log, err := logger.New(level, "json")
			require.NoError(t, err)
			assert.NotNil(t, log)
		})
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	_, err := logger.New("turbo", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "turbo")
}

func TestNew_InvalidFormat(t *testing.T) {
	_, err := logger.New("info", "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml")
}

func TestNop(t *testing.T) {
	log := logger.Nop()
	require.NotNil(t, log)
	// Nop logger should not panic
	log.Info("this should be silently discarded")
	log.Error("this too")
}
