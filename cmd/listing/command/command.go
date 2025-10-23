package command

import (
	listingServer "github.com/all-in-one/cmd/listing/server"
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
			opts := listingServer.Opts{}
			server := listingServer.New(opts)

			return server.Start()
		},
	}

	root := Root()
	root.AddCommand(cmd)

	return root
}
