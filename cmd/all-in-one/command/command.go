package command

import (
	"fmt"

	seed "github.com/all-in-one/cmd/all-in-one/db"
	"github.com/all-in-one/cmd/all-in-one/oidcclient"
	server "github.com/all-in-one/cmd/all-in-one/server"
	tokencli "github.com/all-in-one/cmd/all-in-one/token"
	"github.com/all-in-one/cmd/all-in-one/userimport"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/logging"
	"github.com/all-in-one/internal/oidc/model"
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
--confirm is always required because both directions write to a live database.

Examples:
  # SQLite → PostgreSQL
  all-in-one db:transfer --direction sqlite-to-pg --confirm

  # PostgreSQL → SQLite
  all-in-one db:transfer --direction pg-to-sqlite --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if transferDirection == "" {
				return fmt.Errorf("--direction is required: use sqlite-to-pg or pg-to-sqlite")
			}
			if !transferConfirm {
				return fmt.Errorf("--confirm is required: this will write to the destination database and cannot be undone")
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
	transferCmd.Flags().BoolVar(&transferConfirm, "confirm", false, "required for all transfers — confirms you intend to write to the destination database")

	root := Root()
	root.AddCommand(serverCmd)
	root.AddCommand(seedCmd)
	root.AddCommand(migrateCmd)
	root.AddCommand(transferCmd)
	root.AddCommand(tokenCommands()...)
	root.AddCommand(oidcClientCommands()...)
	root.AddCommand(usersImportCommand())

	return root
}

// tokenCommands builds the app-token management CLI (external rate limiting):
// token:create / token:list / token:revoke.
func tokenCommands() []*cobra.Command {
	loadCtx := func() (config.Config, tokencli.Opts, error) {
		cfg, err := config.Load()
		if err != nil {
			return config.Config{}, tokencli.Opts{}, fmt.Errorf("failed to load config: %w", err)
		}
		log, err := logging.New(cfg.Logging)
		if err != nil {
			return config.Config{}, tokencli.Opts{}, fmt.Errorf("failed to initialize logger: %w", err)
		}
		return *cfg, tokencli.Opts{Config: *cfg, Logger: log}, nil
	}

	var app, name, scope string
	createCmd := &cobra.Command{
		Use:   "token:create",
		Short: "mint an app token for the external rate-limit API",
		Long:  "Mint an app token another service uses to call POST /api/v1/ratelimit/check. The plaintext is printed once to stdout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == "" || name == "" {
				return fmt.Errorf("--app and --name are required")
			}
			_, opts, err := loadCtx()
			if err != nil {
				return err
			}
			return tokencli.RunCreate(cmd.Context(), opts, app, name, scope)
		},
	}
	createCmd.Flags().StringVar(&app, "app", "", "consumer app name, e.g. cashflow (required)")
	createCmd.Flags().StringVar(&name, "name", "", "human label, e.g. 'cashflow prod' (required)")
	createCmd.Flags().StringVar(&scope, "scope", "", "target-key scope prefix (default '<app>.')")

	listCmd := &cobra.Command{
		Use:   "token:list",
		Short: "list app tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, opts, err := loadCtx()
			if err != nil {
				return err
			}
			return tokencli.RunList(cmd.Context(), opts)
		},
	}

	revokeCmd := &cobra.Command{
		Use:   "token:revoke <id>",
		Short: "revoke an app token by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, opts, err := loadCtx()
			if err != nil {
				return err
			}
			return tokencli.RunRevoke(cmd.Context(), opts, args[0])
		},
	}

	return []*cobra.Command{createCmd, listCmd, revokeCmd}
}

// oidcClientCommands manage the apps allowed to log users in through aio
// (OpenID Connect, RFC-001): oidc:client:create / list / update / revoke.
func oidcClientCommands() []*cobra.Command {
	load := func() (oidcclient.Opts, error) {
		cfg, err := config.Load()
		if err != nil {
			return oidcclient.Opts{}, fmt.Errorf("failed to load config: %w", err)
		}
		log, err := logging.New(cfg.Logging)
		if err != nil {
			return oidcclient.Opts{}, fmt.Errorf("failed to initialize logger: %w", err)
		}
		return oidcclient.Opts{Config: *cfg, Logger: log}, nil
	}

	var in model.CreateClientInput
	createCmd := &cobra.Command{
		Use:   "oidc:client:create",
		Short: "register an app to log users in through aio",
		Long:  "Register an OpenID Connect client. The client secret is printed once to stdout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := load()
			if err != nil {
				return err
			}
			return oidcclient.RunCreate(cmd.Context(), opts, in)
		},
	}
	createCmd.Flags().StringVar(&in.ID, "id", "", "client id, e.g. cashflow (required)")
	createCmd.Flags().StringVar(&in.Name, "name", "", "display name shown on aio's login page (required)")
	createCmd.Flags().StringArrayVar(&in.RedirectURIs, "redirect-uri", nil, "allowed callback URL (repeatable, required)")
	createCmd.Flags().StringArrayVar(&in.PostLogoutRedirectURIs, "post-logout-redirect-uri", nil, "where logout may return to (repeatable)")
	createCmd.Flags().StringVar(&in.BrandColor, "brand-color", "", "the app's colour on aio's login pages, e.g. '#0f766e'")
	createCmd.Flags().StringVar(&in.Icon, "icon", "", "an emoji or 1-2 characters shown before the app's name, e.g. 💰")
	_ = createCmd.MarkFlagRequired("id")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("redirect-uri")

	listCmd := &cobra.Command{
		Use:   "oidc:client:list",
		Short: "list apps that log in through aio",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := load()
			if err != nil {
				return err
			}
			return oidcclient.RunList(cmd.Context(), opts)
		},
	}

	var upd model.UpdateClientInput
	updateCmd := &cobra.Command{
		Use:   "oidc:client:update <id>",
		Short: "change an app's name or branding on aio's login pages",
		Long:  "Change an OpenID Connect client's display name, brand colour or icon. Only the flags you pass change; pass an empty value (--icon '') to clear one.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := load()
			if err != nil {
				return err
			}
			flags := cmd.Flags()
			in := model.UpdateClientInput{}
			if flags.Changed("name") {
				in.Name = upd.Name
			}
			if flags.Changed("brand-color") {
				in.BrandColor = upd.BrandColor
			}
			if flags.Changed("icon") {
				in.Icon = upd.Icon
			}
			if in == (model.UpdateClientInput{}) {
				return fmt.Errorf("nothing to change: pass --name, --brand-color or --icon")
			}
			return oidcclient.RunUpdate(cmd.Context(), opts, args[0], in)
		},
	}
	upd.Name, upd.BrandColor, upd.Icon = new(string), new(string), new(string)
	updateCmd.Flags().StringVar(upd.Name, "name", "", "display name")
	updateCmd.Flags().StringVar(upd.BrandColor, "brand-color", "", "colour, e.g. '#0f766e' ('' to clear)")
	updateCmd.Flags().StringVar(upd.Icon, "icon", "", "emoji or 1-2 characters ('' to clear)")

	revokeCmd := &cobra.Command{
		Use:   "oidc:client:revoke <id>",
		Short: "revoke an app's ability to log in through aio",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := load()
			if err != nil {
				return err
			}
			return oidcclient.RunRevoke(cmd.Context(), opts, args[0])
		},
	}
	return []*cobra.Command{createCmd, listCmd, updateCmd, revokeCmd}
}

// usersImportCommand moves cashflow's existing accounts into aio so they can
// log in to cashflow through aio with their current passwords (RFC-001 §7.3).
func usersImportCommand() *cobra.Command {
	var apply bool
	var linkExisting []string
	cmd := &cobra.Command{
		Use:   "users:import",
		Short: "import cashflow's existing accounts into aio and link them",
		Long: `Copy cashflow's local accounts (username + bcrypt password hash, as-is) into aio,
then link each one in cashflow (users.aio_user_id), so users log in to cashflow
through aio with the password they already have. Cashflow's data stays put.

Reads cashflow's database URL from ` + userimport.CashflowDSNEnv + ` (an env var, so the
password stays out of the process list). Without --apply it only prints the plan.
--apply writes nothing while any account is skipped; re-running is safe.

Skipped accounts need a decision: a username aio already has (rename one side,
or --link-existing <username> if it is the same person), aio's bootstrap admin
username, or aio's shared demo account.

Examples:
  export ` + userimport.CashflowDSNEnv + `='postgres://cashflow:...@db:5432/cashflow?sslmode=disable'
  all-in-one users:import                       # dry run
  all-in-one users:import --link-existing opan  # opan in cashflow is opan in aio
  all-in-one users:import --apply`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			log, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			return userimport.RunImport(cmd.Context(), userimport.Opts{
				Config: *cfg, Logger: log, Apply: apply, LinkExisting: linkExisting,
			})
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "create the aio accounts and link them in cashflow (default: dry run)")
	cmd.Flags().StringSliceVar(&linkExisting, "link-existing", nil, "usernames whose existing aio account is the same person (comma-separated)")
	return cmd
}
