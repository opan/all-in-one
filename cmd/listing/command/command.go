package command

import (
	"fmt"

	listingServer "github.com/all-in-one/cmd/listing/server"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/logging"
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "listing",
		Short: "🚀 Listing Service",
		Long:  "",
	}

	return root
}

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start listing server",
		Long:  "start listing server",
		RunE: func(cmd *cobra.Command, args []string) error {

			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			opts := listingServer.Opts{
				Config: *cfg,
				Logger: log,
			}
			server := listingServer.New(opts)

			return server.Start()
		},
	}

	root := Root()
	root.AddCommand(cmd)

	return root
}
