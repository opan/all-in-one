package handler

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/oklog/ulid/v2"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var reservedCodes = map[string]struct{}{
	"api": {}, "r": {}, "admin": {}, "login": {}, "static": {},
	"swagger": {}, "health": {}, "favicon": {}, "assets": {},
}

func newULID() string {
	return ulid.Make().String()
}

func newShortCode(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(base62Chars))))
		if err != nil {
			return "", err
		}
		b[i] = base62Chars[n.Int64()]
	}
	return string(b), nil
}

func isReserved(code string) bool {
	_, ok := reservedCodes[strings.ToLower(code)]
	return ok
}

func isUniqueConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return false
}
