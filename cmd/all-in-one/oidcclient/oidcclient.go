package oidcclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc/model"
	oidcSvc "github.com/all-in-one/internal/oidc/service"
	"github.com/all-in-one/internal/storage"
	"github.com/rs/zerolog"
)

type Opts struct {
	Config config.Config
	Logger zerolog.Logger
}

func registry(opts Opts) (*oidcSvc.Service, error) {
	store, err := storage.NewStorage(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	return oidcSvc.NewClientRegistry(store.DB(), opts.Config, opts.Logger)
}

// RunCreate registers an app and prints its client secret to stdout only, so
// `... | pbcopy` captures exactly the secret; everything else goes to stderr.
func RunCreate(ctx context.Context, opts Opts, in model.CreateClientInput) error {
	svc, err := registry(opts)
	if err != nil {
		return err
	}
	c, secret, err := svc.CreateClient(ctx, in, "cli")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Registered client %q (%s)\n", c.ID, c.Name)
	fmt.Fprintf(os.Stderr, "  redirect URIs:     %s\n", strings.Join(c.RedirectURIs, ", "))
	if len(c.PostLogoutRedirectURIs) > 0 {
		fmt.Fprintf(os.Stderr, "  post-logout URIs:  %s\n", strings.Join(c.PostLogoutRedirectURIs, ", "))
	}
	if b := brandingSummary(c); b != "-" {
		fmt.Fprintf(os.Stderr, "  branding:          %s\n", b)
	}
	if opts.Config.Auth.OIDC.Issuer != "" {
		fmt.Fprintf(os.Stderr, "  issuer:            %s\n", opts.Config.Auth.OIDC.Issuer)
	}
	fmt.Fprintln(os.Stderr, "Store this client secret now; it will not be shown again:")
	fmt.Println(secret)
	return nil
}

func RunList(ctx context.Context, opts Opts) error {
	svc, err := registry(opts)
	if err != nil {
		return err
	}
	clients, err := svc.ListClients(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tBRANDING\tREDIRECT URIS\tCREATED\tREVOKED")
	for _, c := range clients {
		revoked := "-"
		if c.RevokedAt != nil {
			revoked = c.RevokedAt.UTC().Format(time.DateTime)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", c.ID, c.Name, brandingSummary(c), strings.Join(c.RedirectURIs, ","),
			c.CreatedAt.UTC().Format(time.DateTime), revoked)
	}
	return tw.Flush()
}

func RunUpdate(ctx context.Context, opts Opts, id string, in model.UpdateClientInput) error {
	svc, err := registry(opts)
	if err != nil {
		return err
	}
	c, err := svc.UpdateClient(ctx, id, in)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "updated client %q: name %q, branding %s\n", c.ID, c.Name, brandingSummary(c))
	return nil
}

func brandingSummary(c model.Client) string {
	parts := []string{}
	for _, p := range []string{c.Icon, c.BrandColor} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func RunRevoke(ctx context.Context, opts Opts, id string) error {
	svc, err := registry(opts)
	if err != nil {
		return err
	}
	if err := svc.RevokeClient(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "revoked client %q\n", id)
	return nil
}
