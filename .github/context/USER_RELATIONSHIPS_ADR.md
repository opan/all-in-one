# Chat User Relationships - Architecture Decision Record

**Date**: February 21, 2026  
**Status**: ✅ **DECIDED** - Option 2 (Invite-Based System)  
**Decided By**: Product Owner

---

## Decision

We will implement **Option 2: Invite-Based System** from the User Relationships design options.

---

## Context

Currently, the chat feature allows any user to start a chat with any other user. This presents scalability and privacy concerns:

1. **Scalability**: User search loads all users - won't scale beyond a few hundred users
2. **Privacy**: No control over who can initiate conversations
3. **Spam Prevention**: Need mechanism to control unwanted chat requests

Three options were evaluated:
- **Option 1**: Friend Request System (full-featured, like LinkedIn)
- **Option 2**: Invite-Based System (simpler, integrated with chat)
- **Option 3**: Hybrid Approach (maximum flexibility)

---

## Decision Rationale

### Why Option 2 (Invite-Based System)?

#### Advantages:
1. **Simplicity**: Reuses existing chat infrastructure - no separate "connection" concept
2. **Faster Implementation**: ~10-12 hours vs 20-24 hours for full relationship system
3. **Unified UX**: Request and first message combined into single action
4. **Lower Complexity**: Fewer states to manage (no "pending" limbo state)
5. **Natural Flow**: User wants to chat → sends message → recipient accepts → chat begins

#### What We Get:
- ✅ Solves scalability (search only shows "connected" users after first accept)
- ✅ Privacy control (accept/ignore incoming chat requests)
- ✅ Spam prevention (ignore unwanted requests)
- ✅ Simple user experience
- ✅ Admin bypass capability

#### What We Trade Off:
- ⚠️ First message sent before acceptance (slightly less privacy)
- ⚠️ No granular "connection" vs "chat" separation
- ⚠️ Cannot "connect" without starting a chat

### Why Not Option 1?
- More complex - requires separate connection management
- Longer implementation time
- Extra step before chatting (send request → wait → chat)
- Requires notification system for requests

### Why Not Option 3?
- Over-engineered for current needs
- Too many states to manage
- Can add complexity later if needed

---

## Implementation Plan

### Phase 1: Core Invite System (~8 hours)

#### Database Schema
```sql
CREATE TABLE chat_invites (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  from_user_id TEXT NOT NULL,
  to_user_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'accepted', 'ignored'
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  responded_at TIMESTAMP NULL,
  
  FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
  FOREIGN KEY (from_user_id) REFERENCES users(id),
  FOREIGN KEY (to_user_id) REFERENCES users(id)
);

CREATE INDEX idx_chat_invites_to_user ON chat_invites(to_user_id, status);
CREATE INDEX idx_chat_invites_session ON chat_invites(session_id);
```

#### Backend Changes
1. **CreateSession** handler:
   - Create session as "pending" if recipient hasn't accepted yet
   - Create invite record for each new participant
   - Allow sender to send messages immediately
   
2. **GetSessions** handler:
   - Show "accepted" sessions normally
   - Show "pending outgoing" sessions (you invited others)
   - Show "pending incoming" sessions separately (invites you received)

3. **Accept/Ignore Endpoints**:
   - `POST /api/v1/chat-invites/:id/accept` - Accept invite and join session
  - `POST /api/v1/chat-invites/:id/ignore` - Ignore invite (soft delete)

4. **SearchUsers** update:
   - Instead of showing all users, show:
     - Users you have accepted chats with
     - (Optionally) All users if no restrictions desired initially

#### Frontend Changes
1. **Pending Invites Section**: Tab/section showing incoming chat invites
2. **Invite Banner**: Show banner in chat view when viewing pending incoming chat
3. **Accept/Ignore Buttons**: Allow accepting or ignoring invites
4. **Visual States**: Different styling for pending vs active chats

---

### Phase 2: Migration Strategy (~2 hours)

#### Auto-Accept Existing Chats
All existing chat sessions will be auto-marked as "accepted":

```sql
-- Mark all existing sessions as accepted (no invite needed)
-- This is done by NOT creating invite records for existing sessions
-- Existing sessions continue to work normally
```

#### Feature Flag
Add configuration to toggle invite system:

```yaml
chat:
  require_invite_acceptance: true  # Enable invite system
  admin_bypass_invites: true       # Admins don't need acceptance
```

---

### Phase 3: Admin Controls (~2 hours)

#### Admin Bypass
Admins can:
- Start chats with anyone (no invite needed)
- See all users in search
- Join any existing chat session

```go
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
    user := getUserFromContext(ctx)
    
    // Admins skip invite system
    if user.Role == "admin" || user.Role == "superadmin" {
        // Create session directly as "accepted"
        session := createAcceptedSession(participants)
    } else {
        // Regular users create pending session with invites
        session, invites := createPendingSession(participants)
    }
}
```

---

## Future Enhancements (Post-MVP)

Can be added later without breaking changes:

1. **Notifications**:
   - Real-time invite notifications via WebSocket
   - Email notifications for invites
   - Push notifications (if mobile app exists)

2. **Block Feature**:
   - Allow blocking users
   - Blocked users cannot send new invites
   - Existing chats can be deleted

3. **Invite Expiry**:
   - Auto-expire old invites (e.g., 30 days)
   - Periodic cleanup job

4. **Rich Invites**:
   - Add custom message with invite
   - Show preview of first message

5. **Connection Graph**:
   - "Friends" who have mutual accepted chats
   - "Suggested conversations" based on connections

---

## Success Metrics

### Must Have (Phase 1)
- ✅ Users can send chat invites
- ✅ Users can accept/ignore invites
- ✅ Accepted chats work normally
- ✅ Admins bypass system
- ✅ Search shows connected users

### Nice to Have (Phase 2+)
- Notification count for pending invites
- Email notifications
- Block functionality

---

## Rollout Plan

### Week 1: Development
- Day 1-2: Database schema + migration
- Day 3-4: Backend API endpoints
- Day 5: Frontend invite UI

### Week 2: Testing & Polish
- Day 1-2: Testing + bug fixes
- Day 3: Migration of existing data
- Day 4: Documentation
- Day 5: Deploy to staging

### Week 3: Production
- Gradual rollout with feature flag
- Monitor usage and errors
- Collect user feedback

---

## Alternatives Considered

### Alternative 1: No Relationship System
**Rejected** - Doesn't solve scalability or privacy concerns

### Alternative 2: Full Friend Request System (Option 1)
**Rejected** - Too complex for current needs, can add later if needed

### Alternative 3: Hybrid Approach (Option 3)
**Rejected** - Over-engineered, unnecessary complexity

---

## Dependencies

### Required:
- None - can implement immediately

### Optional:
- Notification system (for better UX)
- Admin role checking (for bypass)

---

## Risks & Mitigation

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Users confused by pending state | Medium | Low | Clear UI indicators, help text |
| Spam via invites | High | Medium | Rate limiting, block feature |
| Migration breaks existing chats | High | Low | Thorough testing, rollback plan |
| Performance issues with invite checks | Medium | Low | Proper indexing, caching |

---

## References

- [USER_RELATIONSHIPS_DESIGN.md](./USER_RELATIONSHIPS_DESIGN.md) - Full design document
- [CHAT_PROGRESS.md](./CHAT_PROGRESS.md) - Current chat implementation status
- [CHAT_IMPLEMENTATION_PLAN.md](./CHAT_IMPLEMENTATION_PLAN.md) - Original chat plan

---

## Decision History

- **2026-02-21**: Option 2 (Invite-Based System) selected for initial implementation
- Rationale: Balance between functionality and implementation speed
- Can enhance to Option 1 later if user feedback warrants it

---

## Next Steps

1. ✅ Record decision (this document)
2. ⏳ Create database migration file
3. ⏳ Implement backend repositories
4. ⏳ Implement backend handlers
5. ⏳ Create frontend invite UI
6. ⏳ Test end-to-end flow
7. ⏳ Deploy to staging environment

**Estimated Start Date**: TBD  
**Estimated Completion**: 2 weeks from start
