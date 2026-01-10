-- Chat sessions table
CREATE TABLE IF NOT EXISTS chat_sessions (
  id TEXT PRIMARY KEY UNIQUE,
  parties TEXT NOT NULL, -- comma-separated user IDs (e.g., "uuid1,uuid2,uuid3")
  status TEXT NOT NULL DEFAULT 'active', -- active, archived, deleted
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by TEXT NOT NULL, -- user_id who created the session
  FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Chat messages table
CREATE TABLE IF NOT EXISTS chats (
  id TEXT PRIMARY KEY UNIQUE,
  chat_session_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chat_session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Index for faster queries
CREATE INDEX IF NOT EXISTS idx_chats_session_id ON chats(chat_session_id);
CREATE INDEX IF NOT EXISTS idx_chats_created_at ON chats(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_parties ON chat_sessions(parties);
