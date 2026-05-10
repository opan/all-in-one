package service

import (
	"crypto/rand"
	"math/big"
	"strings"

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
