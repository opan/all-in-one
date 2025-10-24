package server

import (
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/zerolog"
)

type server struct {
	config config.Config
	log    zerolog.Logger
}

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

func New(opts Opts) *server {
	return &server{
		config: opts.Config,
		log:    opts.Logger,
	}
}

func (s *server) Start() error {
	// Implementation of server start logic

	s.log.Info().Msg("Initiating server start...")

	storage := storage.NewStorage(s.config, s.log)

	return nil
}
