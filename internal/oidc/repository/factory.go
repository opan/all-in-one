package repository

import (
	"github.com/all-in-one/internal/config"
	"github.com/all-in-one/internal/oidc/repository/postgres"
	"github.com/all-in-one/internal/oidc/repository/sqlite"
	"github.com/jmoiron/sqlx"
)

type storeAdapter struct {
	clientRepo ClientRepository
	keyRepo    KeyRepository
}

func (a *storeAdapter) ClientRepo() ClientRepository { return a.clientRepo }
func (a *storeAdapter) KeyRepo() KeyRepository       { return a.keyRepo }

func NewRepo(db *sqlx.DB, cfg config.Config) (Storage, error) {
	switch cfg.Storage.Type {
	case "sqlite":
		return &storeAdapter{clientRepo: sqlite.NewClientRepository(db), keyRepo: sqlite.NewKeyRepository(db)}, nil
	case "postgres":
		return &storeAdapter{clientRepo: postgres.NewClientRepository(db), keyRepo: postgres.NewKeyRepository(db)}, nil
	default:
		panic("unsupported storage type: " + cfg.Storage.Type)
	}
}
