package server

import "github.com/all-in-one/internal/config"

type ListingServer struct{}
type Opts struct {
	Config config.Config
}

func New(opts Opts) *ListingServer {
	return &ListingServer{}
}

func (s *ListingServer) Start() error {
	// Implementation of server start logic
	return nil
}
