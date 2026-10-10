package userimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	authnzRepo "github.com/all-in-one/internal/authnz/repository"
	"github.com/all-in-one/internal/authnz/userimport"
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/zerolog"
)

// CashflowDSNEnv names the variable holding cashflow's database URL. It is
// read from the environment, not a flag, so the password stays out of the
// process list and shell history.
const CashflowDSNEnv = "CASHFLOW_DATABASE_URL"

type Opts struct {
	Config       config.Config
	Logger       zerolog.Logger
	Apply        bool
	LinkExisting []string
}

// RunImport moves cashflow's local accounts into aio and links them back
// (RFC-001 §7.3). Without Apply it only reports what it would do.
func RunImport(ctx context.Context, opts Opts) error {
	dsn := os.Getenv(CashflowDSNEnv)
	if dsn == "" {
		return fmt.Errorf("%s is not set: export cashflow's database URL first", CashflowDSNEnv)
	}
	cf, err := userimport.OpenCashflow(ctx, dsn)
	if err != nil {
		return err
	}
	defer cf.Close()

	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	aio, err := authnzRepo.NewRepo(store.DB(), opts.Config)
	if err != nil {
		return fmt.Errorf("failed to create authnz repository: %w", err)
	}

	records, err := cf.Accounts(ctx)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		fmt.Println("Nothing to import: every cashflow account with a password is already linked to aio.")
		return nil
	}

	results, runErr := userimport.Run(ctx, aio.UserRepo(), aio.SessionRepo(), records, userimport.Options{
		Apply:        opts.Apply,
		LinkExisting: opts.LinkExisting,
		Reserved:     reserved(opts.Config.RBAC.AdminUsername, "aio's bootstrap admin, granted the admin group when aio has none"),
		Blocked:      reserved(opts.Config.DemoMode.Username, "aio's shared demo account, which can't log in to other apps"),
	})
	if results != nil {
		printReport(os.Stdout, results)
	}
	skipped := count(results, userimport.Skipped)
	switch {
	case errors.Is(runErr, userimport.ErrSkipped):
		return fmt.Errorf("%d accounts need a decision (see NOTE); nothing was written", skipped)
	case runErr != nil:
		return runErr
	case !opts.Apply:
		if skipped > 0 {
			fmt.Printf("\nDry run: nothing was written. Resolve the %d skipped accounts, then re-run with --apply.\n", skipped)
		} else {
			fmt.Println("\nDry run: nothing was written. Re-run with --apply to import and link these accounts.")
		}
		return nil
	}

	linked, err := cf.Link(ctx, results)
	if err != nil {
		return fmt.Errorf("aio accounts are in place, but linking in cashflow failed (re-running is safe): %w", err)
	}
	fmt.Printf("\nImported into aio: %d new, %d already there, %d linked to existing accounts. Linked in cashflow: %d.\n",
		count(results, userimport.Created), count(results, userimport.AlreadyImported),
		count(results, userimport.LinkedExisting), linked)
	if left, err := cf.Unlinked(ctx); err == nil && left == 0 {
		fmt.Println("Every cashflow account is linked: cashflow can switch to AUTH_PROVIDER=aio.")
	} else if err == nil {
		fmt.Printf("%d cashflow accounts are still not linked; re-run to see why.\n", left)
	}
	return nil
}

func reserved(username, reason string) map[string]string {
	if username = strings.TrimSpace(username); username == "" {
		return nil
	}
	return map[string]string{strings.ToLower(username): reason}
}

func count(results []userimport.Result, o userimport.Outcome) int {
	n := 0
	for _, r := range results {
		if r.Outcome == o {
			n++
		}
	}
	return n
}

func printReport(w io.Writer, results []userimport.Result) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "USERNAME\tRESULT\tAIO ACCOUNT\tNOTE")
	for _, r := range results {
		aioID := "-"
		if r.Outcome != userimport.Skipped {
			aioID = r.AioID.String()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Record.Username, r.Outcome, aioID, r.Reason)
	}
	tw.Flush()
}
