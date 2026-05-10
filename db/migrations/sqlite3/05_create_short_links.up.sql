CREATE TABLE short_links (
    id               TEXT PRIMARY KEY,
    code             TEXT NOT NULL UNIQUE,
    target_url       TEXT NOT NULL,
    owner_id         INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       DATETIME,
    is_active        BOOLEAN NOT NULL DEFAULT 1,
    click_count      INTEGER NOT NULL DEFAULT 0,
    last_accessed_at DATETIME
);

CREATE INDEX idx_short_links_owner_id ON short_links(owner_id);
CREATE INDEX idx_short_links_expires  ON short_links(expires_at) WHERE expires_at IS NOT NULL;
