package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringList is a []string stored as a JSON array in a TEXT column, so the
// same schema works on SQLite and Postgres.
type StringList []string

func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		l = StringList{}
	}
	b, err := json.Marshal([]string(l))
	return string(b), err
}

func (l *StringList) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	case nil:
		*l = StringList{}
		return nil
	default:
		return fmt.Errorf("StringList: unsupported type %T", src)
	}
	return json.Unmarshal(raw, (*[]string)(l))
}

// Client is an app registered to log users in through aio. SecretHash is the
// SHA-256 of a 256-bit random secret: high-entropy, so a fast hash is safe
// (same reasoning as rate-limit app tokens), and it is never serialized.
type Client struct {
	ID                     string     `json:"id" db:"id"`
	Name                   string     `json:"name" db:"name"`
	SecretHash             string     `json:"-" db:"secret_hash"`
	RedirectURIs           StringList `json:"redirect_uris" db:"redirect_uris"`
	PostLogoutRedirectURIs StringList `json:"post_logout_redirect_uris" db:"post_logout_redirect_uris"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
	CreatedBy              *string    `json:"created_by,omitempty" db:"created_by"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// SigningKey is an RSA key used to sign ID tokens. The private key is stored
// as PEM encrypted with auth.totp_encryption_key; only the public half ever
// leaves aio (via the keys endpoint).
type SigningKey struct {
	ID                  string     `db:"id"`
	Algorithm           string     `db:"algorithm"`
	PrivateKeyEncrypted string     `db:"private_key_encrypted"`
	CreatedAt           time.Time  `db:"created_at"`
	RetiredAt           *time.Time `db:"retired_at"`
}

// AuthRequestInfo is what aio's login page needs to render an auth request:
// which app is asking, and whether it asked for the signup form.
type AuthRequestInfo struct {
	ID         string `json:"id"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	Signup     bool   `json:"signup"`
}

// CreateClientInput is what an admin supplies when registering an app.
type CreateClientInput struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	RedirectURIs           []string `json:"redirect_uris"`
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris"`
}
