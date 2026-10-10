// Package userimport moves existing accounts from another first-party app
// (cashflow) into aio, so they can log in to that app through aio's OpenID
// Connect provider with the passwords they already have (RFC-001 §7.3).
//
// Both apps store plain bcrypt hashes, so a hash is copied as-is: nobody
// resets or re-registers. Accepting pre-hashed passwords is only safe from
// a trusted operator, which is why this is a CLI and never an API endpoint.
package userimport

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Record is one account to import, as read from the app's database.
type Record struct {
	ID           string    `db:"id"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

type Outcome string

const (
	// Created: a new aio account with the record's username and hash.
	Created Outcome = "created"
	// AlreadyImported: aio has this username with the same hash (an earlier run).
	AlreadyImported Outcome = "already_imported"
	// LinkedExisting: the operator said the aio account with this username is
	// the same person (--link-existing); its password is left alone.
	LinkedExisting Outcome = "linked_existing"
	// Skipped: needs the operator's decision; see Result.Reason.
	Skipped Outcome = "skipped"
)

type Result struct {
	Record  Record
	Outcome Outcome
	AioID   uuid.UUID // zero when skipped
	Reason  string
}

// Options for a run. Both maps go from a lowercase username to the reason.
// Reserved names are never created from an import but may be linked to an
// existing aio account (e.g. aio's bootstrap admin, which aio would grant the
// admin group); Blocked names can't be imported or linked at all (e.g. the
// shared demo account, which can't log in to other apps).
type Options struct {
	Apply        bool
	LinkExisting []string
	Reserved     map[string]string
	Blocked      map[string]string
}

// Users is the part of aio's user repository the import needs.
type Users interface {
	GetAll(ctx context.Context) ([]model.User, error)
	Create(ctx context.Context, user model.User, opts ...query.QueryOptions) error
}

// TxBeginner starts the transaction every created account is written in.
type TxBeginner interface {
	CreateTrx(ctx context.Context) (query.QueryOptions, error)
}

var (
	// cashflow's username rule; aio itself accepts any non-empty username.
	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

	ErrSkipped = errors.New("some accounts need a decision first")
)

// Run plans the import of every record and, with Apply, creates the new aio
// accounts in one transaction. Apply refuses to write anything while any
// record is skipped, so an import is never left half done.
func Run(ctx context.Context, users Users, tx TxBeginner, records []Record, opts Options) ([]Result, error) {
	existing, err := users.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list aio users: %w", err)
	}
	link, err := linkSet(opts.LinkExisting, records)
	if err != nil {
		return nil, err
	}

	byName := map[string]model.User{}
	byLowerName := map[string][]model.User{}
	for _, u := range existing {
		byName[u.Username] = u
		byLowerName[strings.ToLower(u.Username)] = append(byLowerName[strings.ToLower(u.Username)], u)
	}

	results := make([]Result, 0, len(records))
	seenIDs, seenNames := map[string]bool{}, map[string]bool{}
	for _, rec := range records {
		res := plan(rec, link, byName, byLowerName, opts, seenIDs, seenNames)
		seenIDs[rec.ID], seenNames[rec.Username] = true, true
		results = append(results, res)
	}

	if !opts.Apply {
		return results, nil
	}
	for _, r := range results {
		if r.Outcome == Skipped {
			return results, ErrSkipped
		}
	}
	return results, create(ctx, users, tx, results)
}

func plan(rec Record, link map[string]bool, byName map[string]model.User, byLowerName map[string][]model.User,
	opts Options, seenIDs, seenNames map[string]bool) Result {
	skip := func(reason string) Result { return Result{Record: rec, Outcome: Skipped, Reason: reason} }

	if _, err := uuid.Parse(rec.ID); err != nil {
		return skip("id is not a UUID")
	}
	if seenIDs[rec.ID] || seenNames[rec.Username] {
		return skip("listed twice in the source")
	}
	if !usernamePattern.MatchString(rec.Username) {
		return skip("username doesn't follow cashflow's rule (a-z, 0-9, _, 3-30 characters)")
	}
	if _, err := bcrypt.Cost([]byte(rec.PasswordHash)); err != nil {
		return skip("password_hash is not a bcrypt hash")
	}

	lower := strings.ToLower(rec.Username)
	if reason, ok := opts.Blocked[lower]; ok {
		return skip(reason + ": rename it in cashflow first")
	}
	if u, ok := byName[rec.Username]; ok {
		switch {
		case u.PasswordHash == rec.PasswordHash:
			return Result{Record: rec, Outcome: AlreadyImported, AioID: u.ID}
		case link[rec.Username]:
			return Result{Record: rec, Outcome: LinkedExisting, AioID: u.ID}
		default:
			return skip("aio already has this username for another account: rename one side, " +
				"or pass --link-existing " + rec.Username + " if it is the same person")
		}
	}
	if similar := byLowerName[lower]; len(similar) > 0 {
		if link[rec.Username] && len(similar) == 1 {
			return Result{Record: rec, Outcome: LinkedExisting, AioID: similar[0].ID}
		}
		return skip(fmt.Sprintf("aio has a similar username %q: rename one side, or pass --link-existing %s "+
			"if it is the same person", similar[0].Username, rec.Username))
	}
	if link[rec.Username] {
		return skip("--link-existing was given, but aio has no account with this username")
	}
	if reason, ok := opts.Reserved[lower]; ok {
		return skip("reserved in aio (" + reason + "): rename it in cashflow first")
	}
	return Result{Record: rec, Outcome: Created, AioID: uuid.New()}
}

func linkSet(names []string, records []Record) (map[string]bool, error) {
	inExport := map[string]bool{}
	for _, r := range records {
		inExport[r.Username] = true
	}
	set := map[string]bool{}
	var unknown []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !inExport[n] {
			unknown = append(unknown, n)
		}
		set[n] = true
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("--link-existing names not among the accounts to import: %s", strings.Join(unknown, ", "))
	}
	return set, nil
}

func create(ctx context.Context, users Users, tx TxBeginner, results []Result) error {
	trx, err := tx.CreateTrx(ctx)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Outcome != Created {
			continue
		}
		u := model.User{ID: r.AioID, Username: r.Record.Username, PasswordHash: r.Record.PasswordHash}
		if err := users.Create(ctx, u, trx); err != nil {
			_ = trx.Rollback()
			return fmt.Errorf("create aio account %q (nothing was imported): %w", r.Record.Username, err)
		}
	}
	return trx.Commit()
}
