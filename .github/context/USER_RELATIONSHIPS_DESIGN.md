# User Relationships Feature Design

**Date**: February 21, 2026  
**Status**: 📋 Planning Phase

## Problem Statement

### Current Issues
1. **Scalability**: User search returns all users - won't scale with thousands of users
2. **Privacy**: Any user can initiate chat with anyone - no privacy control
3. **Spam**: No mechanism to prevent unwanted chat requests
4. **No Relationship Model**: Users have no concept of "connections" or "contacts"

### Proposed Solution

Implement a **lightweight relationship system** where users must have a connection before they can chat.

---

## Design Approach

### Option 1: Friend Request System (Recommended)

**Flow**:
1. User A searches for User B
2. User A sends a "connection request" to User B
3. User B receives notification and can Accept/Reject
4. If accepted, both users can now chat
5. Search only shows users you're connected with

**Pros**:
- Familiar UX (like LinkedIn, Facebook)
- Clear user control
- Prevents spam
- Good privacy

**Cons**:
- Extra step before chatting
- Requires notification system

---

### Option 2: Invite-Based System (Simpler)

**Flow**:
1. User A sends chat message to User B (creates pending session)
2. User B sees "pending chat" notification
3. User B can Accept (chat opens) or Ignore (chat deleted)
4. Once accepted, normal chat continues

**Pros**:
- Simpler implementation (reuse chat infrastructure)
- Fewer database tables
- Combined invite + first message

**Cons**:
- First message sent before acceptance (might feel invasive)
- Less granular control

---

### Option 3: Hybrid Approach (Flexible)

**Flow**:
1. Users can be in one of these relationship states:
   - **No Relation**: Can send connection request
   - **Pending**: Request sent, awaiting response
   - **Connected**: Can chat freely
   - **Blocked**: Cannot interact
2. Admins bypass all restrictions
3. Users can have "open chat" setting (accept all requests automatically)

**Pros**:
- Most flexible
- Admin support built-in
- User can choose openness level

**Cons**:
- More complex to implement
- More states to manage

---

## Recommended Implementation (Option 1)

### Database Schema

#### New Table: `user_relationships`

```sql
CREATE TABLE user_relationships (
  id TEXT PRIMARY KEY UNIQUE,
  user_id TEXT NOT NULL,          -- User who initiated
  related_user_id TEXT NOT NULL,  -- User who received request
  status TEXT NOT NULL,            -- 'pending', 'accepted', 'blocked'
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  responded_at TIMESTAMP NULL,    -- When accepted/rejected
  
  -- Ensure same relationship doesn't exist twice
  UNIQUE(user_id, related_user_id),
  
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (related_user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for fast lookups
CREATE INDEX idx_user_relationships_user_id ON user_relationships(user_id);
CREATE INDEX idx_user_relationships_related_user_id ON user_relationships(related_user_id);
CREATE INDEX idx_user_relationships_status ON user_relationships(status);
```

**Note**: Relationship is directional. User A sends request to User B:
- Row exists with `user_id = A, related_user_id = B, status = 'pending'`
- When B accepts, status becomes 'accepted'
- Searching checks BOTH directions: `(A->B OR B->A) AND status = 'accepted'`

---

### Backend Changes

#### 1. New Repository Interface

```go
// internal/chat/repository/interfaces.go

type RelationshipRepository interface {
    // SendRequest creates a new connection request
    SendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID) error
    
    // AcceptRequest accepts a pending request
    AcceptRequest(ctx context.Context, requestID string) error
    
    // RejectRequest deletes a pending request
    RejectRequest(ctx context.Context, requestID string) error
    
    // BlockUser blocks a user
    BlockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error
    
    // GetConnections returns all accepted connections for a user
    GetConnections(ctx context.Context, userID uuid.UUID) ([]UserSearchResult, error)
    
    // GetPendingRequests returns all pending requests (sent TO this user)
    GetPendingRequests(ctx context.Context, userID uuid.UUID) ([]ConnectionRequest, error)
    
    // GetRelationshipStatus checks relationship between two users
    GetRelationshipStatus(ctx context.Context, userID, otherUserID uuid.UUID) (string, error)
    
    // IsConnected checks if two users are connected
    IsConnected(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
}

type ConnectionRequest struct {
    ID            string    `json:"id" db:"id"`
    UserID        string    `json:"user_id" db:"user_id"`
    RelatedUserID string    `json:"related_user_id" db:"related_user_id"`
    Status        string    `json:"status" db:"status"`
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
    Username      string    `json:"username,omitempty" db:"username"` // From JOIN
}
```

#### 2. Update SearchUsers

```go
// OLD: Search all users
func (s *sqliteStorage) SearchUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]UserSearchResult, error) {
    // Returns ALL users matching query
}

// NEW: Search only connected users
func (s *sqliteStorage) SearchConnectedUsers(ctx context.Context, query string, userID uuid.UUID, limit int) ([]UserSearchResult, error) {
    sqlQuery := `
        SELECT DISTINCT u.id, u.username, u.email, u.name
        FROM users u
        INNER JOIN user_relationships r ON (
            (r.user_id = ? AND r.related_user_id = u.id) OR
            (r.related_user_id = ? AND r.user_id = u.id)
        )
        WHERE r.status = 'accepted'
          AND u.id != ?
          AND (u.username LIKE ? OR u.name LIKE ? OR u.email LIKE ?)
        ORDER BY u.username
        LIMIT ?
    `
    // Execute with userID twice for both directions
}

// Keep old function for admins or "add connection" feature
func (s *sqliteStorage) SearchAllUsers(ctx context.Context, query string, excludeUserID uuid.UUID, limit int) ([]UserSearchResult, error) {
    // Original implementation
}
```

#### 3. New API Endpoints

```
POST   /api/v1/connections                  - Send connection request
GET    /api/v1/connections                  - Get all connections
GET    /api/v1/connections/pending          - Get pending requests
POST   /api/v1/connections/:id/accept       - Accept request
POST   /api/v1/connections/:id/reject       - Reject request
DELETE /api/v1/connections/:id              - Remove connection
POST   /api/v1/connections/:id/block        - Block user

GET    /api/v1/users/search                 - Search ALL users (for sending requests)
GET    /api/v1/users/search/connected       - Search connected users only (for chat)
```

---

### Frontend Changes

#### 1. New Components

**`ConnectionRequestDialog.svelte`**: Send request to new user
- Search all users
- Send connection request with optional message
- Shows pending status

**`ConnectionsList.svelte`**: Manage connections
- List of accepted connections
- Pending requests (with accept/reject buttons)
- Remove connection option

**`PendingBadge.svelte`**: Show pending request count
- Display notification count
- Link to connections page

#### 2. Update NewChatDialog

```svelte
<!-- OLD: Search all users -->
searchUsers(query)

<!-- NEW: Search only connected users -->
searchConnectedUsers(query)
```

#### 3. New Route: `/connections`

Page to manage all connections:
- Tab 1: Your Connections (accepted)
- Tab 2: Pending Requests (incoming)
- Tab 3: Sent Requests (outgoing)
- "Add Connection" button

---

## Migration Strategy

### Phase 1: Add Relationship Table (Non-Breaking)
- Create `user_relationships` table
- Add new repository methods
- **Don't enforce yet** - still allow chatting with anyone

### Phase 2: Dual Mode (Transition)
- Add feature flag: `require_connection_for_chat`
- When disabled (default): old behavior
- When enabled: enforce connections
- Admins always bypass

### Phase 3: Auto-Connect Existing Chat Participants
```sql
-- Create relationships for users who already have chat sessions together
INSERT INTO user_relationships (id, user_id, related_user_id, status, created_at)
SELECT 
  LOWER(HEX(RANDOMBLOB(16))),
  csp1.user_id,
  csp2.user_id,
  'accepted',
  CURRENT_TIMESTAMP
FROM chat_session_participants csp1
JOIN chat_session_participants csp2 ON csp1.session_id = csp2.session_id
WHERE csp1.user_id < csp2.user_id  -- Prevent duplicates
  AND NOT EXISTS (
    SELECT 1 FROM user_relationships r
    WHERE (r.user_id = csp1.user_id AND r.related_user_id = csp2.user_id)
       OR (r.user_id = csp2.user_id AND r.related_user_id = csp1.user_id)
  );
```

### Phase 4: Enable Enforcement
- Flip feature flag to `true`
- Monitor and adjust

---

## Admin/Superadmin Exceptions

### Approach 1: Check User Role
```go
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
    user := getUserFromContext(ctx)
    
    // Admins can search all users
    if user.Role == "admin" || user.Role == "superadmin" {
        return h.storage.SearchAllUsers(ctx, query, userID, limit)
    }
    
    // Regular users only see connections
    return h.storage.SearchConnectedUsers(ctx, query, userID, limit)
}
```

### Approach 2: Permission-Based
```go
// Check if user has "chat:unrestricted" permission
if hasPermission(user, "chat:unrestricted") {
    // Can chat with anyone
}
```

---

## Configuration

Add to `config/config.yml`:

```yaml
chat:
  require_connection: true      # Enforce relationship system
  admin_bypass: true            # Admins can chat with anyone
  auto_accept_from_admin: true  # Admin requests auto-accepted
  max_connections: 1000         # Limit per user
  request_expiry_days: 30       # Auto-reject old requests
```

---

## User Experience

### Happy Path: New User
1. Login to app
2. Go to "Connections" page
3. Search for colleague by name
4. Click "Send Request"
5. (Optional) Add message: "Hi! Let's connect"
6. Wait for acceptance
7. Once accepted, can chat

### Existing Chats (Migration)
- All existing chat participants automatically connected
- Users notice no change
- Can continue chatting seamlessly

### Edge Cases
- **Request already exists**: Show "Request Pending"
- **User blocks you**: Shows "Unable to connect"
- **You blocked them**: Don't show in search
- **Admin searches**: Sees everyone, bypass required

---

## Implementation Estimate

### Backend
- Migration file: **30 minutes**
- Repository layer: **2 hours**
- Handler endpoints: **2 hours**
- Middleware for enforcement: **1 hour**
- **Total**: ~5-6 hours

### Frontend
- Connection request dialog: **2 hours**
- Connections page: **3 hours**
- Update chat search: **1 hour**
- Notification badge: **1 hour**
- **Total**: ~7 hours

### Testing
- Backend unit tests: **2 hours**
- Integration tests: **2 hours**
- E2E tests: **2 hours**
- Manual testing: **2 hours**
- **Total**: ~8 hours

**Grand Total**: ~20-24 hours of development

---

## Alternative: Simple Approach for MVP

If full feature is too much, start with:

### Minimal Viable Relationship (MVR)

1. **No pending requests** - all requests auto-accepted
2. **One-way connection** - User A adds User B, instant chat access
3. **Remove connection** - Either user can remove
4. **Admin bypass** - Built-in from day 1

This reduces:
- No notification system needed
- No accept/reject flow
- No pending state
- Just "Add to Connections" button

**Benefits**:
- 50% less code
- Faster to ship
- Still solves scalability
- Still gives some privacy control

Implementation: ~10-12 hours instead of 20-24 hours

---

## Recommendation

Start with **MVR** (Minimal Viable Relationship):
1. Implement relationship table
2. Auto-accept all requests (no pending state)
3. Update search to use connections
4. Admin bypass built-in
5. "Add Connection" flow in UI

Then enhance later:
- Add pending/accept flow
- Add notifications
- Add blocking
- Add request messages

---

## Next Steps

1. Discuss approach with team
2. Decide: Full feature or MVR?
3. Create tickets/stories
4. Prioritize against other features
5. Schedule implementation

**Question for you**: 
- Do you want full relationship system, or start with MVR?
- Should we fix the selection bug first, then tackle this?
- Any specific use cases I'm missing?
