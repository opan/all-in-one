CREATE TABLE short_links (
    id               TEXT PRIMARY KEY,
    code             TEXT NOT NULL UNIQUE,
    target_url       TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMP,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    click_count      INTEGER NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMP
);

CREATE TABLE short_link_owners (
    code    TEXT NOT NULL PRIMARY KEY REFERENCES short_links(code) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_short_links_expires       ON short_links(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_short_link_owners_user_id ON short_link_owners(user_id);
