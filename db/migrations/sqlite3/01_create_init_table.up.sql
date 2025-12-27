CREATE TABLE IF NOT EXISTS topics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  form_schema TEXT, -- JSON Form Schema in JSON format
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  topic_id INTEGER,
  title TEXT NOT NULL,
  description TEXT,
  form_schema_values TEXT, -- values for JSON Form Schema in JSON format
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (topic_id) REFERENCES topics(id)
);

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY UNIQUE,
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL UNIQUE,
  name TEXT UNIQUE,
  password_hash TEXT NOT NULL,
  last_login TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY UNIQUE,
  user_id TEXT NOT NULL,
  created_at TIMESTAMP,
  user_agent TEXT,
  access_token_expiry INTEGER, -- unix timestampt
  refresh_token_expiry INTEGER, -- unix timestampt
  FOREIGN KEY (user_id) REFERENCES users(id)
);
