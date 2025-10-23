package server

type ListingServer struct{}
type Opts struct{}

func New(opts Opts) *ListingServer {
	return &ListingServer{}
}

func (s *ListingServer) Start() error {
	// Implementation of server start logic
	return nil
}
