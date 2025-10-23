package command

import (
	listingServer "github.com/all-in-one/cmd/listing/server"
	"github.com/all-in-one/internal/config"
	"github.com/sirupsen/logrus"
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
			cfg, err := config.LoadConfig()
			if err != nil {
				logrus.Fatalf("Failed to load config: %v", err)
			}

			opts := listingServer.Opts{
				Config: *cfg,
			}
			server := listingServer.New(opts)

			return server.Start()
		},
	}

	root := Root()
	root.AddCommand(cmd)

	return root
}
