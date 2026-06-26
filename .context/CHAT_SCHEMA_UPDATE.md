# Chat Feature Database Schema Update

## Date: February 19, 2026

## Overview
Updated the chat feature database schema to properly support participant tracking, session uniqueness, and auto-cleanup functionality as per the agreed-upon requirements.

## Requirements Implemented

### Core Behaviors:
1. **Session Uniqueness**: Sessions are uniquely identified by their participant set (order doesn't matter)
2. **Add Participants**: Adding participants modifies existing session
3. **Leave/Rejoin**: Users can leave and rejoin sessions with access to history
4. **Auto-cleanup**: Sessions auto-delete when fewer than 2 active participants remain

## Database Schema Changes

### Migration: `02_create_chat_tables` (Updated)

#### 1. **chat_sessions** table
**Changes:**
- Removed: `parties TEXT` (deprecated comma-separated approach)
- Added: `participant_hash TEXT NOT NULL UNIQUE` - SHA256 hash of sorted participant IDs
- New index: `idx_chat_sessions_participant_hash` for fast session lookups
- New index: `idx_chat_sessions_created_by`

**Structure:**
```sql
CREATE TABLE chat_sessions (
  id TEXT PRIMARY KEY UNIQUE,
  participant_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by TEXT NOT NULL,
  FOREIGN KEY (created_by) REFERENCES users(id)
);
```

#### 2. **chat_session_participants** table (NEW)
**Purpose:** Track individual participants with join/leave timestamps

**Structure:**
```sql
CREATE TABLE chat_session_participants (
  session_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  left_at TIMESTAMP NULL,  -- NULL = active, NOT NULL = left
  PRIMARY KEY (session_id, user_id),
  FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
```

**Indexes:**
- `idx_chat_session_participants_user_id` - Query user's sessions
- `idx_chat_session_participants_session_active` - Filter active participants

#### 3. **chat_messages** table (Renamed from `chats`)
**Changes:**
- Renamed table: `chats` → `chat_messages` (better naming)
- Added: `sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
- Updated all related indexes

**Structure:**
```sql
CREATE TABLE chat_messages (
  id TEXT PRIMARY KEY UNIQUE,
  chat_session_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chat_session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

**Indexes:**
- `idx_chat_messages_session_id`
- `idx_chat_messages_created_at`
- `idx_chat_messages_sent_at`

## Go Model Updates

### 1. **ChatSession** (`internal/chat/model/session.go`)
**Changes:**
- Removed: `Parties string` field
- Added: `ParticipantHash string` - unique identifier
- Added: `Participants []SessionParticipant` - populated by queries

**New Methods:**
- `GetActiveParticipants()` - returns only active participants
- `GetActiveParticipantIDs()` - returns IDs of active participants
- `GenerateParticipantHash([]string)` - creates hash from participant IDs

**Removed:** All deprecated party-related methods

### 2. **SessionParticipant** (NEW)
```go
type SessionParticipant struct {
    SessionID string
    UserID    string
    JoinedAt  time.Time
    LeftAt    *time.Time  // NULL = active
    Username  string      // Populated via JOIN
}
```

**Method:**
- `IsActive()` - checks if `LeftAt` is NULL

### 3. **ChatMessage** (`internal/chat/model/message.go`)
**Changes:**
- Renamed: `Chat` → `ChatMessage`
- Added: `SentAt time.Time`

## Repository Updates

### SessionRepository Interface
**New Methods:**
- `GetByParticipantHash(ctx, hash)` - Find session by participant set
- `GetParticipants(ctx, sessionID)` - Get all participants
- `AddParticipant(ctx, sessionID, userID)` - Add/reactivate participant
- `RemoveParticipant(ctx, sessionID, userID)` - Mark as left, auto-delete if needed
- `GetActiveParticipantCount(ctx, sessionID)` - Count active participants

**Updated Methods:**
- `GetAllByUserID()` - Now uses JOIN with participants table
- `Get()` - Loads participants automatically
- `Create()` - Creates session + participants in transaction

### MessageRepository Interface
**Changes:**
- Updated all methods to use `ChatMessage` instead of `Chat`
- SQL queries updated to use `chat_messages` table
- Added `sent_at` field handling

## Handler Updates

### Key Changes:
1. **Participant Checking:** Replaced `HasParty()` calls with loop checking `GetActiveParticipants()`
2. **Session Creation:** 
   - Now checks for existing session by `participant_hash`
   - Returns existing session if found (prevents duplicates)
   - Creates participants in same transaction
3. **Add Participants:** Uses `AddParticipant()` repository method
4. **Message Types:** All `model.Chat` → `model.ChatMessage`

### Request Models:
- `CreateSessionRequest.Parties` → `CreateSessionRequest.Participants`
- `UpdateSessionRequest.Parties` → `UpdateSessionRequest.AddParticipants`

## Features Enabled

### ✅ Implemented:
1. Session uniqueness by participant set (hash-based)
2. Participant join/leave tracking
3. Rejoin capability with history access
4. Auto-delete sessions with <2 active participants
5. Transaction-safe session creation
6. Proper foreign key cascades

### 🔄 Ready for Implementation:
1. Leave session endpoint (uses `RemoveParticipant()`)
2. List session participants endpoint
3. Kick participant functionality (admin-based)

## Testing Recommendations

1. **Session Creation:**
   - Create session with 2+ participants
   - Verify participant_hash generated correctly
   - Attempt duplicate creation → should return existing session

2. **Participant Management:**
   - Add participant to session
   - Remove participant → verify auto-delete if <2 remain
   - Rejoin after leaving → verify history access

3. **Messages:**
   - Send/receive messages as active participant
   - Verify non-participants cannot access
   - Check message ordering by `sent_at`

## Migration Instructions

If you have existing data:
1. Backup database
2. Drop old chat tables:
   ```sql
   DROP TABLE IF EXISTS chats;
   DROP TABLE IF EXISTS chat_sessions;
   ```
3. Run migrations: Server will apply on startup via `store.Migrate()`

For fresh install:
- Migrations apply automatically on first server start

## Notes

- Participant hash uses SHA256 of sorted participant IDs for consistency
- All participant operations use transactions for data integrity
- Cascade deletes ensure orphaned records are cleaned up
- Username populated via JOIN for performance
