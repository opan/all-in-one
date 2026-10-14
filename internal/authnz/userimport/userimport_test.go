package userimport

import (
	"context"
	"errors"
	"testing"

	"github.com/all-in-one/internal/authnz/model"
	"github.com/all-in-one/internal/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type fakeTx struct{ committed, rolledBack bool }

func (t *fakeTx) Commit() error   { t.committed = true; return nil }
func (t *fakeTx) Rollback() error { t.rolledBack = true; return nil }

// fakeAio is aio's user store: Create writes into pending until the
// transaction commits, like the real repository does.
type fakeAio struct {
	users   []model.User
	pending []model.User
	tx      *fakeTx
	failOn  string
}

func (f *fakeAio) GetAll(context.Context) ([]model.User, error) { return f.users, nil }

func (f *fakeAio) Create(_ context.Context, u model.User, opts ...query.QueryOptions) error {
	if len(opts) != 1 || opts[0] != f.tx {
		return errors.New("created outside the import transaction")
	}
	if u.Username == f.failOn {
		return errors.New("insert failed")
	}
	f.pending = append(f.pending, u)
	return nil
}

func (f *fakeAio) CreateTrx(context.Context) (query.QueryOptions, error) {
	f.tx = &fakeTx{}
	return f.tx, nil
}

func (f *fakeAio) created() []model.User {
	if f.tx != nil && f.tx.committed {
		return f.pending
	}
	return nil
}

func hash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func rec(username, passwordHash string) Record {
	return Record{ID: uuid.NewString(), Username: username, PasswordHash: passwordHash}
}

func TestRun_PlansEachAccount(t *testing.T) {
	budiHash := hash(t, "budi-password")
	opanAio := model.User{ID: uuid.New(), Username: "opan", PasswordHash: hash(t, "aio-password")}
	sitiAio := model.User{ID: uuid.New(), Username: "siti", PasswordHash: hash(t, "siti-old")}
	rinaAio := model.User{ID: uuid.New(), Username: "Rina", PasswordHash: hash(t, "rina")}
	budiAio := model.User{ID: uuid.New(), Username: "budi", PasswordHash: budiHash}
	aio := &fakeAio{users: []model.User{opanAio, sitiAio, rinaAio, budiAio}}

	records := []Record{
		rec("budi", budiHash),                                // imported by an earlier run
		rec("opan", hash(t, "cashflow-pw")),                  // same person, --link-existing
		rec("siti", hash(t, "different")),                    // clash with another aio account
		rec("rina", hash(t, "x")),                            // aio has "Rina"
		rec("tono", hash(t, "tono")),                         // new
		rec("admin", hash(t, "a")),                           // aio's bootstrap admin
		rec("demo", hash(t, "d")),                            // aio's shared demo account
		rec("Bad Name", hash(t, "b")),                        // not a cashflow username
		rec("plain", "not-a-bcrypt-hash"),                    // corrupt hash
		{ID: "42", Username: "noid", PasswordHash: budiHash}, // id isn't a UUID
	}
	results, err := Run(context.Background(), aio, aio, records, Options{
		LinkExisting: []string{"opan"},
		Reserved:     map[string]string{"admin": "bootstrap admin"},
		Blocked:      map[string]string{"demo": "shared demo account"},
	})
	require.NoError(t, err, "a dry run reports skips without failing")

	got := map[string]Result{}
	for _, r := range results {
		got[r.Record.Username] = r
	}
	assert.Equal(t, AlreadyImported, got["budi"].Outcome)
	assert.Equal(t, budiAio.ID, got["budi"].AioID)
	assert.Equal(t, LinkedExisting, got["opan"].Outcome)
	assert.Equal(t, opanAio.ID, got["opan"].AioID)
	assert.Equal(t, Created, got["tono"].Outcome)
	assert.NotEqual(t, uuid.Nil, got["tono"].AioID)

	skips := map[string]string{
		"siti":     "aio already has this username",
		"rina":     `similar username "Rina"`,
		"admin":    "bootstrap admin",
		"demo":     "shared demo account",
		"Bad Name": "cashflow's rule",
		"plain":    "not a bcrypt hash",
		"noid":     "not a UUID",
	}
	for name, why := range skips {
		assert.Equal(t, Skipped, got[name].Outcome, name)
		assert.Contains(t, got[name].Reason, why, name)
	}
	assert.Nil(t, aio.tx, "a dry run never opens a transaction")
}

func TestRun_LinkExistingCases(t *testing.T) {
	rinaAio := model.User{ID: uuid.New(), Username: "Rina", PasswordHash: hash(t, "r")}
	adminAio := model.User{ID: uuid.New(), Username: "admin", PasswordHash: hash(t, "a")}
	demoAio := model.User{ID: uuid.New(), Username: "demo", PasswordHash: hash(t, "d")}
	aio := &fakeAio{users: []model.User{rinaAio, adminAio, demoAio}}
	records := []Record{rec("rina", hash(t, "x")), rec("admin", hash(t, "y")), rec("demo", hash(t, "z")), rec("nobody", hash(t, "n"))}

	results, err := Run(context.Background(), aio, aio, records, Options{
		LinkExisting: []string{"rina", "admin", "demo", "nobody"},
		Reserved:     map[string]string{"admin": "bootstrap admin"},
		Blocked:      map[string]string{"demo": "shared demo account"},
	})
	require.NoError(t, err)
	assert.Equal(t, LinkedExisting, results[0].Outcome, "a case-only difference can be linked explicitly")
	assert.Equal(t, rinaAio.ID, results[0].AioID)
	assert.Equal(t, LinkedExisting, results[1].Outcome, "the operator may link the reserved admin name to its real owner")
	assert.Equal(t, Skipped, results[2].Outcome, "the shared demo account is never linked")
	assert.Equal(t, Skipped, results[3].Outcome)
	assert.Contains(t, results[3].Reason, "aio has no account with this username")
}

func TestRun_LinkExistingTypoIsAnError(t *testing.T) {
	_, err := Run(context.Background(), &fakeAio{}, &fakeAio{}, []Record{rec("budi", hash(t, "b"))},
		Options{LinkExisting: []string{"budy"}})
	assert.ErrorContains(t, err, "budy")
}

func TestRun_DuplicatesInSource(t *testing.T) {
	r := rec("budi", hash(t, "b"))
	results, err := Run(context.Background(), &fakeAio{}, &fakeAio{}, []Record{r, r}, Options{})
	require.NoError(t, err)
	assert.Equal(t, Created, results[0].Outcome)
	assert.Equal(t, Skipped, results[1].Outcome)
}

func TestRun_ApplyCreatesInOneTransaction(t *testing.T) {
	h := hash(t, "tono-password")
	aio := &fakeAio{}
	results, err := Run(context.Background(), aio, aio, []Record{rec("tono", h), rec("budi", hash(t, "b"))}, Options{Apply: true})
	require.NoError(t, err)
	require.Len(t, aio.created(), 2)
	tono := aio.created()[0]
	assert.Equal(t, "tono", tono.Username)
	assert.Equal(t, h, tono.PasswordHash, "the hash is copied as-is")
	assert.Equal(t, results[0].AioID, tono.ID, "the reported id is the one created")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(tono.PasswordHash), []byte("tono-password")),
		"the user's existing password works in aio")
}

func TestRun_ApplyWritesNothingWhileAnythingIsSkipped(t *testing.T) {
	aio := &fakeAio{users: []model.User{{ID: uuid.New(), Username: "siti", PasswordHash: hash(t, "other")}}}
	_, err := Run(context.Background(), aio, aio, []Record{rec("tono", hash(t, "t")), rec("siti", hash(t, "s"))}, Options{Apply: true})
	assert.ErrorIs(t, err, ErrSkipped)
	assert.Nil(t, aio.tx)
	assert.Empty(t, aio.created())
}

func TestRun_ApplyRollsBackOnFailure(t *testing.T) {
	aio := &fakeAio{failOn: "budi"}
	_, err := Run(context.Background(), aio, aio, []Record{rec("tono", hash(t, "t")), rec("budi", hash(t, "b"))}, Options{Apply: true})
	assert.ErrorContains(t, err, "nothing was imported")
	assert.True(t, aio.tx.rolledBack)
	assert.Empty(t, aio.created())
}
