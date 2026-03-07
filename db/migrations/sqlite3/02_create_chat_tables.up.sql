-- Chat sessions table
CREATE TABLE IF NOT EXISTS chat_sessions (
  id TEXT PRIMARY KEY UNIQUE,
  participant_hash TEXT NOT NULL UNIQUE, -- SHA256 hash of sorted participant IDs for uniqueness
  status TEXT NOT NULL DEFAULT 'active', -- active, archived, deleted
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by TEXT NOT NULL, -- user_id who created the session
  FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Chat session participants table
CREATE TABLE IF NOT EXISTS chat_session_participants (
  session_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  left_at TIMESTAMP NULL, -- NULL means still active in the session
  PRIMARY KEY (session_id, user_id),
  FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Chat messages table
CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY UNIQUE,
  chat_session_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chat_session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Indexes for faster queries
-- Chat sessions
CREATE INDEX IF NOT EXISTS idx_chat_sessions_participant_hash ON chat_sessions(participant_hash);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_created_by ON chat_sessions(created_by);

-- Chat session participants
CREATE INDEX IF NOT EXISTS idx_chat_session_participants_user_id ON chat_session_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_session_participants_session_active ON chat_session_participants(session_id, left_at);

-- Chat messages
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(chat_session_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_sent_at ON chat_messages(sent_at);
