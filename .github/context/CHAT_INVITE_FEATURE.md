# Chat Invite Feature — Implementation Plan

**Created**: March 7, 2026  
**Status**: Planning

---

## Problem

Currently, when User A creates a chat session, all selected users are immediately added as participants and the session goes active. There is no consent or confirmation from the other parties. This is undesirable because:

- Users receive chats they never agreed to join
- No way to decline unwanted conversations
- Group chats can add people without their approval

## Solution

Introduce a **chat invite flow** where:

1. User A selects participants and **sends an invite** (instead of creating a session directly)
2. Each invitee receives a real-time notification and sees the invite in their pending list
3. Invitees can **accept** or **decline** individually
4. When at least one invitee accepts, a chat session is created with the inviter + accepted users
5. Declined users are not added; they can be re-invited later
6. Existing session participants can **invite new users** into the group — same consent flow applies

---

## Design Decisions

### Session Creation Timing

**Chosen: Create session on first acceptance**

- Invites are standalone records, no session exists until someone accepts
- Avoids polluting the session list with "pending" sessions
- Clean separation: invites are invites, sessions are active conversations
- The `participant_hash` is computed from inviter + first-accepting user (the founding pair)

### `participant_hash` in Invite-Created Sessions

The existing `participant_hash` column enforces DB-level uniqueness on sessions. With the invite system:

- Hash is computed from **inviter + first acceptor** — the two people who founded the session
- Subsequent acceptors join the existing session; the hash does **not** change to reflect all members
- This means the hash identifies the *founding pair*, not the full membership roster
- It still provides a DB-level guard against duplicate session creation for the same founding pair
- Primary duplicate prevention moves to the invite layer (`HasPendingInvite` checks)
- If invite logic has a bug, the hash is the last line of defense against creating a second session for the same two originators

> Using a random UUID instead of a hash would lose DB-level dedup entirely, leaving only application-level protection. Given the existing codebase relies on `GetByParticipantHash`, we keep the hash approach.

### Invite Grouping

For group invites (User A invites B, C, D), each invitee gets an individual invite record linked by a shared `batch_id`. This allows:

- Independent accept/decline per user
- Clear tracking of each invitee's decision
- Simple queries for "my pending invites"

### Re-invite Behavior

If User A invites User B who declines, User A can send a new invite later. Old declined/cancelled invites are preserved for history.

### Invite to Existing Session

An active participant in an existing session can invite additional users into that group:

- The invite is created with `session_id` pre-set (pointing to the existing session)
- On acceptance, the invitee is added as a participant to the existing session (no new session created)
- On decline, nothing changes in the session
- Only active participants of the session can send invites for it
- The invited user must not already be an active participant

Comparison of the two flows:

| | New Session Invite | Existing Session Invite |
|--|--|--|
| `session_id` at creation | `NULL` | Set to existing session ID |
| On first accept | Create session, set `session_id` | Add participant to session |
| On subsequent accept | Add participant to session | Add participant to session |
| Who can invite | Any authenticated user | Active participants of the session only |

### Duplicate Prevention

A user cannot send a new invite to someone who already has a `pending` invite from them for the same context — enforced at application level:
- **New session invites**: check for pending invite from same inviter to same invitee where `session_id IS NULL`
- **Existing session invites**: check for pending invite targeting the same `session_id` for the same invitee

> These must be scoped separately. `HasPendingInvite` must filter `AND session_id IS NULL` for new-session checks, otherwise a pending session-specific invite would incorrectly block a new-session invite between the same two users.

---

## Database Schema

### New Migration: `03_create_chat_invites_table.up.sql`

```sql
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
```

### Down Migration: `03_create_chat_invites_table.down.sql`

```sql
DROP INDEX IF EXISTS idx_chat_invites_invitee_status;
DROP INDEX IF EXISTS idx_chat_invites_status;
DROP INDEX IF EXISTS idx_chat_invites_batch;
DROP INDEX IF EXISTS idx_chat_invites_invitee;
DROP INDEX IF EXISTS idx_chat_invites_inviter;
DROP TABLE IF EXISTS chat_invites;
```

### Column Details

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT PK | Unique invite ID (UUID) |
| `batch_id` | TEXT | Groups invites from the same action (UUID). For 1-on-1 invite, batch has 1 row. For group invite to 3 users, batch has 3 rows. |
| `inviter_id` | TEXT FK | The user who sent the invite |
| `invitee_id` | TEXT FK | The user being invited |
| `session_id` | TEXT FK nullable | For new-session invites: set when a session is created upon first acceptance. For existing-session invites: set at invite creation time (points to the session being invited into). |
| `status` | TEXT | `pending`, `accepted`, `declined`, `cancelled` |
| `created_at` | TIMESTAMP | When the invite was created |
| `updated_at` | TIMESTAMP | Last status change |

---

## Backend Implementation

### Phase 1: Model

**File**: `internal/chat/model/invite.go`

```go
type ChatInvite struct {
    ID        string     `json:"id" db:"id"`
    BatchID   string     `json:"batch_id" db:"batch_id"`
    InviterID string     `json:"inviter_id" db:"inviter_id"`
    InviteeID string     `json:"invitee_id" db:"invitee_id"`
    SessionID *string    `json:"session_id,omitempty" db:"session_id"`
    Status    string     `json:"status" db:"status"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`

    // Populated by joins
    InviterUsername string `json:"inviter_username,omitempty" db:"inviter_username"`
    InviteeUsername string `json:"invitee_username,omitempty" db:"invitee_username"`
}

type CreateInviteRequest struct {
    Participants []string `json:"participants"`          // user IDs to invite
    SessionID    *string  `json:"session_id,omitempty"` // if set, invite into this existing session
}

type RespondInviteRequest struct {
    Action string `json:"action"` // "accept" or "decline"
}
```

Invite statuses:
- `pending` — awaiting response
- `accepted` — invitee accepted, session created/joined
- `declined` — invitee declined
- `cancelled` — inviter cancelled before response

### Phase 2: Repository

**File**: `internal/chat/repository/interfaces.go` — add `InviteRepository` and extend `Storage`

```go
type InviteRepository interface {
    Create(ctx context.Context, invite ChatInvite) (ChatInvite, error)
    GetByID(ctx context.Context, id string) (ChatInvite, error)
    GetPendingByInviteeID(ctx context.Context, inviteeID uuid.UUID) ([]ChatInvite, error)
    GetSentByInviterID(ctx context.Context, inviterID uuid.UUID) ([]ChatInvite, error)
    GetByBatchID(ctx context.Context, batchID string) ([]ChatInvite, error)
    UpdateStatus(ctx context.Context, id string, status string) error
    UpdateBatchSessionID(ctx context.Context, batchID string, sessionID string) error
    HasPendingInvite(ctx context.Context, inviterID, inviteeID string) (bool, error)           // scoped to session_id IS NULL
    HasPendingInviteForSession(ctx context.Context, sessionID, inviteeID string) (bool, error) // scoped to specific session_id
}
```

Also extend the `Storage` interface to expose the invite repository:

```go
// In the Storage interface, add:
InviteRepo() InviteRepository
```

`sqliteStorage` in `repository/sqlite.go` must add an `inviteRepo InviteRepository` field, wire it in `NewSQLiteStorage`, and implement `InviteRepo() InviteRepository`.

**File**: `internal/chat/repository/sqlite/invite.go` — SQLite implementation

> Note: `factory.go` does not need changes — it only calls `NewStorage` which delegates to `NewSQLiteStorage`. Invite repo wiring happens inside `NewSQLiteStorage`.

### ⚠️ Route Registration Order

Gorilla Mux matches routes in registration order — first match wins. `/chats/invites` is 2 path segments, same as `/chats/{id}`. If `/chats/{id}` is registered first, all invite routes will be swallowed with `{id} = "invites"`.

**All `/chats/invites/...` routes must be registered before `/chats/{id}` in `handler.go`.**

Registration order in `RegisterAuthenticatedRoutes`:
```
/chats                          GET, POST
/chats/invites                  POST          ← before {id}
/chats/invites/received         GET           ← before {id}
/chats/invites/sent             GET           ← before {id}
/chats/invites/{id}/respond     POST          ← before {id}
/chats/invites/{id}             DELETE        ← before {id}
/chats/{id}                     GET, PUT, DELETE
/chats/{id}/leave               POST
/chats/{id}/messages            GET, POST
/ws
/users/search
```

### Phase 3: Handlers

**File**: `internal/chat/handler/invite.go`

| Method | Route | Description |
|--------|-------|-------------|
| `CreateInvite` | `POST /api/v1/chats/invites` | Send invite(s) to selected users (new or existing session) |
| `GetReceivedInvites` | `GET /api/v1/chats/invites/received` | Get all pending invites for current user |
| `GetSentInvites` | `GET /api/v1/chats/invites/sent` | Get invites sent by current user |
| `RespondToInvite` | `POST /api/v1/chats/invites/{id}/respond` | Accept or decline an invite |
| `CancelInvite` | `DELETE /api/v1/chats/invites/{id}` | Cancel a pending invite (inviter only) |

#### CreateInvite Flow (New Session)
1. Validate participant UUIDs
2. Check `HasPendingInvite` (scoped to `session_id IS NULL`) for each invitee — reject duplicates
3. Generate `batch_id`
4. Create one invite row per invitee (with `session_id = NULL`)
5. Send real-time WebSocket notification to each invitee (if online)

#### CreateInvite Flow (Existing Session)
1. Validate `session_id` exists and is active
2. Verify current user is an active participant of the session
3. Validate invitee UUIDs; reject any who are already active participants
4. Check `HasPendingInviteForSession` for each invitee — reject duplicates
5. Generate `batch_id`
6. Create one invite row per invitee (with `session_id` pre-set)
7. Send real-time WebSocket notification to each invitee (if online)

#### RespondToInvite (Accept) Flow
1. Verify invite belongs to current user and is `pending`
2. Update invite status to `accepted`
3. Determine session:
   - **Invite has `session_id` (existing session invite)**: add this user as participant to that session
   - **Invite has no `session_id` (new session invite)**: check batch — does a session already exist for this batch?
     - **No session yet**: create session with inviter + this user, compute `participant_hash` from these two, set `session_id` on all batch invites
     - **Session exists**: add this user as participant to existing session
4. Send WebSocket notification to inviter (and other session participants)
5. Return `RespondInviteResponse{Invite: updated, Session: session}`

#### RespondToInvite (Decline) Flow
1. Verify invite belongs to current user and is `pending`
2. Update invite status to `declined`
3. Send WebSocket notification to inviter

### Phase 4: WebSocket Events

New message types to add:

| Type | Direction | Description |
|------|-----------|-------------|
| `invite_received` | Server → Client | Sent to invitee when they receive an invite |
| `invite_accepted` | Server → Client | Sent to inviter when invitee accepts |
| `invite_declined` | Server → Client | Sent to inviter when invitee declines |
| `invite_cancelled` | Server → Client | Sent to invitee when inviter cancels |

Payload for invite events:
```go
type InvitePayload struct {
    InviteID        string `json:"invite_id"`
    BatchID         string `json:"batch_id"`
    InviterID       string `json:"inviter_id"`
    InviterUsername  string `json:"inviter_username"`
    InviteeID       string `json:"invitee_id"`
    InviteeUsername  string `json:"invitee_username"`
    SessionID       string `json:"session_id,omitempty"`
    Status          string `json:"status"`
}
```

### Phase 5: Modify CreateSession

The existing `POST /api/v1/chats` (CreateSession) should be updated:
- **Option A**: Remove it entirely — sessions only created through invite acceptance
- **Option B (recommended)**: Keep it but restrict to internal use / mark as deprecated. The invite flow calls session creation internally when an invite is accepted.

**Recommendation**: Keep `CreateSession` but make it an internal method (unexported or behind a flag). The public-facing flow is always through invites.

---

## Frontend Implementation

### Phase 6: API Client

**File**: `web/src/lib/chat-api.ts` — add invite functions

```typescript
export interface ChatInvite {
    id: string;
    batch_id: string;
    inviter_id: string;
    invitee_id: string;
    session_id: string | null;
    status: string;
    created_at: string;
    updated_at: string;
    inviter_username?: string;
    invitee_username?: string;
}

export interface RespondInviteRequest {
    action: "accept" | "decline";
}

export interface RespondInviteResponse {
    invite: ChatInvite;
    session?: ChatSession; // present only on accept
}

export async function sendInvite(request: { participants: string[]; session_id?: string }): Promise<ChatInvite[]>;
export async function getReceivedInvites(): Promise<ChatInvite[]>;
export async function getSentInvites(): Promise<ChatInvite[]>;
export async function respondToInvite(inviteId: string, request: RespondInviteRequest): Promise<RespondInviteResponse>;
export async function cancelInvite(inviteId: string): Promise<void>;
```

### Phase 7: WebSocket Types

**File**: `web/src/lib/websocket-types.ts` — add invite event types

```typescript
export type WebSocketMessageType = "message" | "join" | "leave" | "typing" | "error"
    | "invite_received" | "invite_accepted" | "invite_declined" | "invite_cancelled";

export interface InvitePayload {
    invite_id: string;
    batch_id: string;
    inviter_id: string;
    inviter_username: string;
    invitee_id: string;
    invitee_username: string;
    session_id?: string;
    status: string;
}
```

### Phase 8: Update NewChatDialog

**File**: `web/src/components/NewChatDialog.svelte`

Changes:
- Button text: "Start Chat" → "Send Invite"
- Calls `sendInvite()` instead of `createSession()`
- Success message: "Invite sent! Waiting for acceptance."
- Dialog closes after sending

### Phase 8b: Add "Invite to Session" UI

**File**: `web/src/routes/chat/+page.svelte` (or new component)

Changes:
- Add an "Invite User" button/menu option in the active chat header (visible to session participants)
- Opens a user search dialog similar to NewChatDialog but with `session_id` pre-set
- Calls `sendInvite({ participants: [...], session_id: activeSessionId })`
- Prevents inviting users already in the session (filter them from search results)

### Phase 9: Invite Inbox UI

**Option A (recommended)**: Invite panel integrated into the chat sidebar  
**Option B**: Separate invites page

For Option A:
- Add a tab or section at the top of the chat sidebar: "Invites (N)"
- Badge count for pending invites
- Clicking shows a list of pending invites with:
  - Inviter username
  - When sent
  - Accept / Decline buttons
- On accept: session appears in session list, auto-selected
- On decline: invite removed from list

### Phase 10: Real-time Invite Notifications

- WebSocket client handles new invite event types
- When `invite_received`: add to invites list, show indicator/badge
- When `invite_accepted`: refresh session list (new session appeared)
- When `invite_declined`: update sent invites list
- Toast notifications for key events

---

## Implementation Order

| Step | Task | Estimated Effort |
|------|------|-----------------|
| 1 | Database migration (create `chat_invites` table) | 30 min |
| 2 | Backend model (`ChatInvite`, request/response types) | 30 min |
| 3 | Backend repository interface + SQLite implementation | 1.5 hrs |
| 4 | Backend handlers (create, respond, list, cancel) | 2 hrs |
| 5 | Route registration + Swagger docs | 30 min |
| 6 | WebSocket invite event types + broadcasting | 1 hr |
| 7 | Modify/restrict direct session creation | 30 min |
| 8 | Frontend API client (invite functions) | 30 min |
| 9 | Frontend WebSocket types + handlers | 30 min |
| 10 | Update NewChatDialog (send invite flow) | 1 hr |
| 11 | Invite inbox UI (sidebar section) | 2 hrs |
| 12 | Real-time notifications + badges | 1.5 hrs |
| 13 | Testing + edge cases | 2 hrs |
| **Total** | | **~14 hrs** |

---

## Edge Cases to Handle

1. **User invites someone who already has a pending invite from them** → reject with `"Invite already pending"`
2. **User invites someone they already have an active session with** → reject with `"A chat session already exists with this user"`. The frontend should surface the existing session instead of letting the user re-create it.
3. **All invitees decline a group invite** → batch stays with no session, inviter can re-invite
4. **Inviter cancels after some acceptances** → remaining pending invites cancelled, session (if created) stays active for already-accepted users
5. **Re-invite after decline** → allowed, creates a new invite record
6. **User goes offline before responding** → invite persists in DB, shown when they return
7. **Concurrent acceptance in group invite** → use transaction to prevent race conditions when creating session / adding participants
8. **Invite into existing session for someone who was already a participant** → reject with "User is already a participant"
9. **Invite into an existing session by a non-participant** → reject with "Not authorized" (only active participants can invite)
10. **Invite into a deleted/archived session** → reject with "Session is not active"

---

## API Endpoint Summary

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/chats/invites` | Send invite(s) — new or existing session | JWT |
| GET | `/api/v1/chats/invites/received` | Get pending received invites | JWT |
| GET | `/api/v1/chats/invites/sent` | Get sent invites | JWT |
| POST | `/api/v1/chats/invites/{id}/respond` | Accept or decline invite | JWT |
| DELETE | `/api/v1/chats/invites/{id}` | Cancel pending invite | JWT |

---

## Files to Create/Modify

### New Files
- `db/migrations/sqlite3/03_create_chat_invites_table.up.sql`
- `db/migrations/sqlite3/03_create_chat_invites_table.down.sql`
- `internal/chat/model/invite.go`
- `internal/chat/repository/sqlite/invite.go`
- `internal/chat/handler/invite.go`

### Modified Files
- `internal/chat/repository/interfaces.go` — add `InviteRepository` interface + `InviteRepo()` to `Storage` interface
- `internal/chat/repository/sqlite.go` — add `inviteRepo` field, wire in `NewSQLiteStorage`, implement `InviteRepo()`
- `internal/chat/handler/handler.go` — register invite routes **before** `/chats/{id}` routes
- `internal/chat/model/message.go` — add `InvitePayload` and `RespondInviteResponse` WebSocket/response types
- `internal/chat/websocket/hub.go` — no changes needed (uses existing `BroadcastToUsers`)
- `web/src/lib/chat-api.ts` — add invite API functions + `RespondInviteResponse` type
- `web/src/lib/websocket-types.ts` — add invite event types
- `web/src/components/NewChatDialog.svelte` — send invite instead of create session
- `web/src/routes/chat/+page.svelte` — invite inbox UI + badge + handlers
