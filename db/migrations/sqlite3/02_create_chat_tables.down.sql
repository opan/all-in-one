-- Drop indexes
DROP INDEX IF EXISTS idx_chat_messages_sent_at;
DROP INDEX IF EXISTS idx_chat_messages_created_at;
DROP INDEX IF EXISTS idx_chat_messages_session_id;
DROP INDEX IF EXISTS idx_chat_session_participants_session_active;
DROP INDEX IF EXISTS idx_chat_session_participants_user_id;
DROP INDEX IF EXISTS idx_chat_sessions_created_by;
DROP INDEX IF EXISTS idx_chat_sessions_participant_hash;

-- Drop tables
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_session_participants;
DROP TABLE IF EXISTS chat_sessions;
