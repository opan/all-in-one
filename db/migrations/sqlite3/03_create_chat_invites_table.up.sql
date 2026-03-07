CREATE TABLE IF NOT EXISTS chat_invites (
  id TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL,
  inviter_id TEXT NOT NULL,
  invitee_id TEXT NOT NULL,
  session_id TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (inviter_id) REFERENCES users(id),
  FOREIGN KEY (invitee_id) REFERENCES users(id),
  FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_chat_invites_inviter ON chat_invites(inviter_id);
CREATE INDEX IF NOT EXISTS idx_chat_invites_invitee ON chat_invites(invitee_id);
CREATE INDEX IF NOT EXISTS idx_chat_invites_batch ON chat_invites(batch_id);
CREATE INDEX IF NOT EXISTS idx_chat_invites_status ON chat_invites(status);
CREATE INDEX IF NOT EXISTS idx_chat_invites_invitee_status ON chat_invites(invitee_id, status);
