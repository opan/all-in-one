CREATE TABLE IF NOT EXISTS oidc_clients (
    id                        TEXT PRIMARY KEY,
    name                      TEXT NOT NULL,
    secret_hash               TEXT NOT NULL,
    redirect_uris             TEXT NOT NULL,
    post_logout_redirect_uris TEXT NOT NULL DEFAULT '[]',
    created_at                TIMESTAMP NOT NULL,
    created_by                TEXT,
    revoked_at                TIMESTAMP
);

CREATE TABLE IF NOT EXISTS oidc_signing_keys (
    id                    TEXT PRIMARY KEY,
    algorithm             TEXT NOT NULL,
    private_key_encrypted TEXT NOT NULL,
    created_at            TIMESTAMP NOT NULL,
    retired_at            TIMESTAMP
);
