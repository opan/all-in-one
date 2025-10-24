package server

import (
	"github.com/all-in-one/internal/config"
	"github.com/rs/zerolog"
)

type listingServer struct {
	config config.Config
	log    zerolog.Logger
}

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

func New(opts Opts) *listingServer {
	return &listingServer{
		config: opts.Config,
		log:    opts.Logger,
	}
}

func (s *listingServer) Start() error {
	// Implementation of server start logic
	return nil
}
