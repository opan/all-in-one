package logging

import (
	"os"

	"github.com/all-in-one/internal/config"
	"github.com/rs/zerolog"
)

func New(cfg config.LoggingConfig) (zerolog.Logger, error) {

	// Setup logging
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return log, err
	}
	zerolog.SetGlobalLevel(level)

	return log, nil
}
