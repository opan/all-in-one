package token

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/all-in-one/internal/config"
	ratelimitSvc "github.com/all-in-one/internal/ratelimit/service"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/zerolog"
)

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

// newService opens the storage and constructs the ratelimit service, which
// owns token creation/verification. The caller must Close the returned service.
func newService(ctx context.Context, opts Opts) (*ratelimitSvc.Service, error) {
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	svc, err := ratelimitSvc.NewService(ctx, store.DB(), opts.Config, opts.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create ratelimit service: %w", err)
	}
	return svc, nil
}

// RunCreate mints a token and prints the plaintext to stdout (only), so
// `... | pbcopy` captures exactly the token. The human-readable notice and
// metadata go to stderr.
func RunCreate(ctx context.Context, opts Opts, app, name, scope string) error {
	svc, err := newService(ctx, opts)
	if err != nil {
		return err
	}
	defer svc.Close()

	tok, plaintext, err := svc.CreateToken(ctx, app, name, scope, "cli")
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Created app token %s for %q (scope %q).\n", tok.ID, tok.App, tok.ScopePrefix)
	fmt.Fprintln(os.Stderr, "Store this token now — it will not be shown again:")
	fmt.Println(plaintext)
	return nil
}

// RunList prints all tokens as a table to stdout.
func RunList(ctx context.Context, opts Opts) error {
	svc, err := newService(ctx, opts)
	if err != nil {
		return err
	}
	defer svc.Close()

	tokens, err := svc.ListTokens(ctx)
	if err != nil {
		return fmt.Errorf("list tokens: %w", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tAPP\tNAME\tPREFIX\tSCOPE\tCREATED\tLAST USED\tREVOKED")
	for _, t := range tokens {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.App, t.Name, t.TokenPrefix, t.ScopePrefix,
			fmtTime(&t.CreatedAt), fmtTime(t.LastUsedAt), fmtTime(t.RevokedAt))
	}
	return tw.Flush()
}

// RunRevoke revokes a token by id.
func RunRevoke(ctx context.Context, opts Opts, id string) error {
	svc, err := newService(ctx, opts)
	if err != nil {
		return err
	}
	defer svc.Close()

	if err := svc.RevokeToken(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	fmt.Fprintf(os.Stderr, "revoked app token %s\n", id)
	return nil
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04")
}
