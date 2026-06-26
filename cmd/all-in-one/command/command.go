package command

import (
	"fmt"

	seed "github.com/all-in-one/cmd/all-in-one/db"
	server "github.com/all-in-one/cmd/all-in-one/server"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/logging"
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "all-in-one",
		Short: "All-in-one server",
		Long:  "All-in-one server",
	}

	return root
}

func New() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "start all-in-one server",
		Long:  "start all-in-one server",
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

			opts := server.Opts{
				Config: *cfg,
				Logger: log,
			}
			svr := server.New(opts)

			return svr.Start()
		},
	}

	seedCmd := &cobra.Command{
		Use:   "db:seed",
		Short: "seed the database with initial data",
		Long:  "Initialize the database with sample users, topics, and items for development and testing",
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

			opts := seed.Opts{
				Config: *cfg,
				Logger: log,
			}

			return seed.Run(opts)
		},
	}

	migrateCmd := &cobra.Command{
		Use:   "db:migrate",
		Short: "run database migrations",
		Long:  "Manage database schema migrations",
	}

	migrateUpCmd := &cobra.Command{
		Use:   "up",
		Short: "apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			return seed.RunMigrateUp(seed.MigrateOpts{
				Config: *cfg,
				Logger: log,
			})
		},
	}

	var downSteps int
	migrateDownCmd := &cobra.Command{
		Use:   "down",
		Short: "roll back migrations",
		Long:  "Roll back migrations. Use --steps to specify how many steps to roll back (default 0 = all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			return seed.RunMigrateDown(seed.MigrateOpts{
				Config: *cfg,
				Logger: log,
			}, downSteps)
		},
	}
	migrateDownCmd.Flags().IntVar(&downSteps, "steps", 0, "number of steps to roll back (0 = all)")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)

	var transferDirection string
	var transferConfirm bool
	transferCmd := &cobra.Command{
		Use:   "db:transfer",
		Short: "migrate data between SQLite and PostgreSQL",
		Long: `Copy all application data from one storage backend to the other.

Both databases must have all schema migrations applied before running.
The destination must be empty; existing rows cause constraint failures.

Examples:
  # SQLite → PostgreSQL
  all-in-one db:transfer --direction sqlite-to-pg

  # PostgreSQL → SQLite  (requires --confirm to protect all-in-one.db)
  all-in-one db:transfer --direction pg-to-sqlite --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if transferDirection == "" {
				return fmt.Errorf("--direction is required: use sqlite-to-pg or pg-to-sqlite")
			}
			if transferDirection == "pg-to-sqlite" && !transferConfirm {
				return fmt.Errorf("writing to SQLite will modify %s — re-run with --confirm to proceed",
					"the configured SQLite database")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			log, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			return seed.RunTransfer(cmd.Context(), seed.TransferOpts{
				Direction: transferDirection,
				Config:    *cfg,
				Logger:    log,
			})
		},
	}
	transferCmd.Flags().StringVar(&transferDirection, "direction", "", "transfer direction: sqlite-to-pg or pg-to-sqlite (required)")
	transferCmd.Flags().BoolVar(&transferConfirm, "confirm", false, "required when direction is pg-to-sqlite to confirm writing to SQLite")

	root := Root()
	root.AddCommand(serverCmd)
	root.AddCommand(seedCmd)
	root.AddCommand(migrateCmd)
	root.AddCommand(transferCmd)

	return root
}
