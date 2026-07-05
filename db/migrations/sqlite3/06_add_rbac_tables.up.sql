CREATE TABLE IF NOT EXISTS features (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    admin_only INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS group_features (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (group_id, feature_id)
);

CREATE TABLE IF NOT EXISTS user_feature_overrides (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
    allow INTEGER NOT NULL, -- 1 = grant, 0 = revoke; absence of a row = inherit from group
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, feature_id)
);

ALTER TABLE users ADD COLUMN group_id TEXT REFERENCES groups(id) ON DELETE SET NULL;

CREATE INDEX idx_group_features_feature_id ON group_features(feature_id);
CREATE INDEX idx_user_feature_overrides_feature_id ON user_feature_overrides(feature_id);
CREATE INDEX idx_users_group_id ON users(group_id);
