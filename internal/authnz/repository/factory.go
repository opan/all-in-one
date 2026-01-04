package repository

import (
	"github.com/all-in-one/internal/authnz/repository/sqlite"
	"github.com/all-in-one/internal/config"
)

func NewStorage(config config.Config) {
	switch config.Storage.Type {
	// case "memory":
	// 	return memory.NewStorage()
	case "sqlite":
		return sqlite.NewStorage(config)
	default:
		panic("unsupported storage type: " + config.Storage.Type)
	}
}
