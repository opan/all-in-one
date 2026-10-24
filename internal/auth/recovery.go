package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
)

// tempPasswordAlphabet has no look-alikes (0/o, 1/l/i) and only lower case,
// so a temporary password read from an email is hard to mistype.
const tempPasswordAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

const TempPasswordLength = 12

// GenerateTempPassword returns a random temporary password (RFC-003 §6.4),
// about 59 bits of entropy.
func GenerateTempPassword() (string, error) {
	b := make([]byte, TempPasswordLength)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = tempPasswordAlphabet[n.Int64()]
	}
	return string(b), nil
}

// NormalizeTempPassword undoes the email's formatting: case, dashes and
// spaces are ignored when a temporary password is typed.
func NormalizeTempPassword(s string) string {
	return strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(s))
}

// LooksLikeTempPassword reports whether s could be a temporary password once
// normalized, so ordinary passwords skip the extra bcrypt comparisons.
func LooksLikeTempPassword(s string) bool {
	n := NormalizeTempPassword(s)
	if len(n) != TempPasswordLength {
		return false
	}
	for _, r := range n {
		if !strings.ContainsRune(tempPasswordAlphabet, r) {
			return false
		}
	}
	return true
}

// GenerateLinkToken returns a random token for an emailed link and the hash
// to store. The token is 256 bits, so a fast hash is enough.
func GenerateLinkToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashLinkToken(token), nil
}

func HashLinkToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
