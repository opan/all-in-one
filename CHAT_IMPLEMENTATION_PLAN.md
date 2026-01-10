# Chat Application Implementation Plan

## Overview
Implement a real-time chat application with WebSocket support following the existing project patterns from `listing` and `authnz` apps.

---

## 1. Database Schema & Migrations

### Migration Files
Create: `db/migrations/sqlite3/02_create_chat_tables.up.sql`

```sql
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
```

Create: `db/migrations/sqlite3/02_create_chat_tables.down.sql`

```sql
DROP INDEX IF EXISTS idx_chat_sessions_parties;
DROP INDEX IF EXISTS idx_chats_created_at;
DROP INDEX IF EXISTS idx_chats_session_id;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS chat_sessions;
```

---

## 2. Project Structure

Create the following structure under `internal/chat/`:

```
internal/chat/
├── handler/
│   ├── handler.go           # Main handler with dependency injection
│   ├── session.go           # Chat session CRUD handlers
│   ├── message.go           # Message handlers (WebSocket)
│   ├── session_test.go      # Tests for session handlers
│   └── message_test.go      # Tests for message handlers
├── model/
│   ├── session.go           # ChatSession model
│   └── message.go           # Chat message model
├── repository/
│   ├── factory.go           # Repository factory (similar to listing)
│   ├── interfaces.go        # Repository interfaces
│   ├── sqlite.go            # SQLite implementation
│   └── sqlite/
│       ├── session.go       # Session repository implementation
│       └── message.go       # Message repository implementation
├── service/
│   └── service.go           # Service layer with dependency setup
├── websocket/
│   ├── hub.go               # WebSocket connection hub/manager
│   ├── client.go            # WebSocket client representation
│   └── message.go           # WebSocket message types
└── seed/
    └── seed.go              # Seed data for testing (optional)
```

---

## 3. Core Models

### `internal/chat/model/session.go`
```go
package model

import (
    "strings"
    "time"
    "github.com/google/uuid"
)

type ChatSession struct {
    ID        string    `json:"id" db:"id"`
    Parties   string    `json:"parties" db:"parties"` // comma-separated
    Status    string    `json:"status" db:"status"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
    CreatedBy string    `json:"created_by" db:"created_by"`
}

// GetPartyIDs returns slice of user IDs
func (cs *ChatSession) GetPartyIDs() []uuid.UUID {
    parts := strings.Split(cs.Parties, ",")
    ids := make([]uuid.UUID, 0, len(parts))
    for _, p := range parts {
        if id, err := uuid.Parse(strings.TrimSpace(p)); err == nil {
            ids = append(ids, id)
        }
    }
    return ids
}

// SetPartyIDs sets parties from UUID slice
func (cs *ChatSession) SetPartyIDs(ids []uuid.UUID) {
    parts := make([]string, len(ids))
    for i, id := range ids {
        parts[i] = id.String()
    }
    cs.Parties = strings.Join(parts, ",")
}
```

### `internal/chat/model/message.go`
```go
package model

import "time"

type Chat struct {
    ID            string    `json:"id" db:"id"`
    ChatSessionID string    `json:"chat_session_id" db:"chat_session_id"`
    UserID        string    `json:"user_id" db:"user_id"`
    Message       string    `json:"message" db:"message"`
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
    
    // Optional: Include username for frontend convenience
    Username      string    `json:"username,omitempty" db:"-"`
}
```

---

## 4. Repository Layer

### `internal/chat/repository/interfaces.go`
```go
package repository

import (
    "context"
    "github.com/all-in-one/internal/chat/model"
    "github.com/google/uuid"
)

type SessionRepository interface {
    // GetAll returns all sessions for a user
    GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]model.ChatSession, error)
    
    // Get returns a session by ID (with permission check)
    Get(ctx context.Context, id string, userID uuid.UUID) (model.ChatSession, error)
    
    // Create creates a new chat session
    Create(ctx context.Context, session model.ChatSession) (model.ChatSession, error)
    
    // Update updates session (e.g., add parties, change status)
    Update(ctx context.Context, id string, session model.ChatSession) (model.ChatSession, error)
    
    // Delete soft deletes a session
    Delete(ctx context.Context, id string) error
    
    // AddParty adds a user to the session
    AddParty(ctx context.Context, sessionID string, userID uuid.UUID) error
}

type MessageRepository interface {
    // GetBySessionID returns all messages for a session
    GetBySessionID(ctx context.Context, sessionID string, limit int) ([]model.Chat, error)
    
    // Create creates a new message
    Create(ctx context.Context, chat model.Chat) (model.Chat, error)
    
    // Get returns a message by ID
    Get(ctx context.Context, id string) (model.Chat, error)
}

type Storage interface {
    SessionRepo() SessionRepository
    MessageRepo() MessageRepository
    Close() error
}
```

---

## 5. WebSocket Implementation

### `internal/chat/websocket/hub.go`
Central hub to manage all WebSocket connections and broadcast messages.

**Key responsibilities:**
- Register/unregister clients
- Broadcast messages to all clients in a chat session
- Handle message routing

```go
package websocket

import (
    "sync"
    "github.com/all-in-one/internal/chat/model"
)

type Hub struct {
    // Registered clients per session
    sessions map[string]map[*Client]bool
    
    // Inbound messages from clients
    broadcast chan *BroadcastMessage
    
    // Register requests from clients
    register chan *Client
    
    // Unregister requests from clients
    unregister chan *Client
    
    mu sync.RWMutex
}

type BroadcastMessage struct {
    SessionID string
    Message   model.Chat
}

func NewHub() *Hub {
    return &Hub{
        sessions:   make(map[string]map[*Client]bool),
        broadcast:  make(chan *BroadcastMessage),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (h *Hub) Run() {
    // Main event loop
}
```

### `internal/chat/websocket/client.go`
Represents individual WebSocket connection.

```go
package websocket

import (
    "github.com/gorilla/websocket"
    "github.com/google/uuid"
)

type Client struct {
    hub       *Hub
    conn      *websocket.Conn
    send      chan []byte
    sessionID string
    userID    uuid.UUID
}

// readPump reads messages from client
func (c *Client) readPump() {}

// writePump writes messages to client
func (c *Client) writePump() {}
```

---

## 6. API Endpoints

### REST Endpoints (HTTP)

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/api/v1/chats` | Get all chat sessions for current user | JWT |
| POST | `/api/v1/chats` | Create new chat session | JWT |
| GET | `/api/v1/chats/{session_id}` | Get session details | JWT |
| PUT | `/api/v1/chats/{session_id}` | Update session (add parties) | JWT |
| DELETE | `/api/v1/chats/{session_id}` | Delete/archive session | JWT |
| GET | `/api/v1/chats/{session_id}/messages` | Get message history | JWT |
| GET | `/api/v1/users/search` | Search users (for inviting) | JWT |

### WebSocket Endpoint
- `ws://localhost:8080/api/v1/chats/{session_id}/ws` - WebSocket connection for real-time messaging

**WebSocket message format:**
```json
{
  "type": "message|join|leave|typing",
  "payload": {
    "message": "Hello!",
    "user_id": "uuid",
    "username": "john_doe"
  },
  "timestamp": "2026-01-10T12:00:00Z"
}
```

---

## 7. Handler Implementation

### `internal/chat/handler/handler.go`
```go
package handler

import (
    "github.com/all-in-one/internal/chat/repository"
    "github.com/all-in-one/internal/chat/websocket"
    "github.com/all-in-one/internal/config"
    "github.com/gorilla/mux"
)

type Handler struct {
    storage repository.Storage
    config  config.Config
    hub     *websocket.Hub
}

func NewHandler(storage repository.Storage, config config.Config, hub *websocket.Hub) *Handler {
    return &Handler{
        storage: storage,
        config:  config,
        hub:     hub,
    }
}

func (h *Handler) RegisterAuthenticatedRoutes(router *mux.Router) {
    // Session management
    router.HandleFunc("/chats", h.GetSessions).Methods("GET")
    router.HandleFunc("/chats", h.CreateSession).Methods("POST")
    router.HandleFunc("/chats/{id}", h.GetSession).Methods("GET")
    router.HandleFunc("/chats/{id}", h.UpdateSession).Methods("PUT")
    router.HandleFunc("/chats/{id}", h.DeleteSession).Methods("DELETE")
    
    // Message endpoints
    router.HandleFunc("/chats/{id}/messages", h.GetMessages).Methods("GET")
    
    // WebSocket endpoint
    router.HandleFunc("/chats/{id}/ws", h.HandleWebSocket)
    
    // User search for invitations
    router.HandleFunc("/users/search", h.SearchUsers).Methods("GET")
}
```

### Key Handler Methods

**`GetSessions`** - Return all sessions where user is a party
**`CreateSession`** - Create new session with initial parties
**`UpdateSession`** - Add parties to existing session
**`GetMessages`** - Get message history with pagination
**`HandleWebSocket`** - Upgrade HTTP to WebSocket connection
**`SearchUsers`** - Search users by username/name for invitations

---

## 8. Service Integration

### `internal/chat/service/service.go`
```go
package service

import (
    "context"
    "fmt"
    "github.com/all-in-one/internal/chat/handler"
    "github.com/all-in-one/internal/chat/repository"
    "github.com/all-in-one/internal/chat/websocket"
    "github.com/all-in-one/internal/config"
    "github.com/gorilla/mux"
    "github.com/rs/zerolog"
)

type Service struct {
    Handler *handler.Handler
    Storage repository.Storage
    Hub     *websocket.Hub
}

func NewService(ctx context.Context, config config.Config, log zerolog.Logger) (*Service, error) {
    store, err := repository.NewStorage(ctx, config, log)
    if err != nil {
        return nil, fmt.Errorf("failed to create storage: %w", err)
    }
    
    hub := websocket.NewHub()
    go hub.Run() // Start hub in background
    
    h := handler.NewHandler(store, config, hub)
    
    return &Service{
        Handler: h,
        Storage: store,
        Hub:     hub,
    }, nil
}

func (s *Service) RegisterAuthenticatedRoutes(router *mux.Router) {
    s.Handler.RegisterAuthenticatedRoutes(router)
}

func (s *Service) Close() error {
    return s.Storage.Close()
}
```

### Update `cmd/all-in-one/server/server.go`
Add chat service initialization:

```go
import chatSvc "github.com/all-in-one/internal/chat/service"

// In Start() method, after listing service:
csvc, err := chatSvc.NewService(ctx, s.config, s.log)
if err != nil {
    s.log.Error().Err(err).Msg("Failed to create chat service")
    return err
}

// Register routes
csvc.RegisterAuthenticatedRoutes(authenticatedRoutes)
```

---

## 9. Dependencies

### Add WebSocket Library
```bash
go get github.com/gorilla/websocket
```

Update `go.mod` to include:
```
github.com/gorilla/websocket v1.5.3
```

---

## 10. Testing Strategy

### Unit Tests
- Handler tests with mocked repositories (use `mockery`)
- Repository tests with in-memory SQLite
- Model validation tests

### Integration Tests
- WebSocket connection tests
- End-to-end message flow tests
- Session creation and management tests

### Example Test Structure
```go
// internal/chat/handler/session_test.go
func TestCreateSession(t *testing.T) {
    // Table-driven tests
    tests := []struct{
        name string
        input CreateSessionRequest
        want Response
        wantErr bool
    }{
        // test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

---

## 11. Frontend UX Flow

### Pages/Routes (Svelte)

1. **Chat List Page** (`/chats`)
   - Display all chat sessions in a table/list
   - Show last message, unread count, timestamp
   - "New Chat" button

2. **Chat Session Page** (`/chats/{id}`)
   - Real-time message display
   - Message input box
   - Participant list with "Invite User" button
   - WebSocket connection for live updates

3. **New Chat Modal/Page**
   - Search and select users to start conversation
   - Create session button

### Components
- `ChatList.svelte` - List of all sessions
- `ChatSession.svelte` - Individual chat view
- `MessageList.svelte` - Display messages
- `MessageInput.svelte` - Input and send messages
- `UserSearch.svelte` - Search users to invite
- `ParticipantList.svelte` - Show session participants

### WebSocket Client (TypeScript)
```typescript
// web/src/lib/websocket.ts
export class ChatWebSocket {
    private ws: WebSocket | null = null;
    
    connect(sessionId: string, token: string) {
        const wsUrl = `ws://localhost:8080/api/v1/chats/${sessionId}/ws`;
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            // Handle incoming message
        };
    }
    
    send(message: string) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({
                type: 'message',
                payload: { message }
            }));
        }
    }
    
    disconnect() {
        this.ws?.close();
    }
}
```

---

## 12. Implementation Checklist

### Phase 1: Backend Foundation
- [ ] Create database migration files (up/down)
- [ ] Run migrations to create tables
- [ ] Create model structs (`session.go`, `message.go`)
- [ ] Implement repository interfaces
- [ ] Implement SQLite repository

### Phase 2: REST API
- [ ] Create handler struct with DI
- [ ] Implement session CRUD handlers
- [ ] Implement message retrieval handler
- [ ] Implement user search handler
- [ ] Add route registration
- [ ] Write handler tests

### Phase 3: WebSocket
- [ ] Install `gorilla/websocket`
- [ ] Implement Hub (connection manager)
- [ ] Implement Client (connection wrapper)
- [ ] Implement WebSocket upgrade handler
- [ ] Add message broadcasting logic
- [ ] Test WebSocket connectivity

### Phase 4: Service Integration
- [ ] Create service.go with initialization
- [ ] Start Hub in background goroutine
- [ ] Integrate with server.go
- [ ] Add to main server startup

### Phase 5: Frontend
- [ ] Create API client methods
- [ ] Implement WebSocket client wrapper
- [ ] Create ChatList page/component
- [ ] Create ChatSession page/component
- [ ] Create MessageInput component
- [ ] Create UserSearch component
- [ ] Add routing

### Phase 6: Testing & Polish
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Add Swagger documentation
- [ ] Add proper error handling
- [ ] Add logging throughout
- [ ] Test end-to-end flow

---

## 13. Recommendations & Improvements

### Security
1. **WebSocket Authentication**: Validate JWT token on WebSocket upgrade
   - Pass token via query parameter or initial message
   - Verify user has permission to join session

2. **Session Access Control**: Verify user is in `parties` before allowing access
   - Add middleware to check session membership
   - Return 403 for unauthorized access

3. **Rate Limiting**: Prevent message spam
   - Implement rate limiting per user/session
   - Use sliding window or token bucket algorithm

### Features
1. **Message Status**: Add read receipts
   - Track which users have read messages
   - Show "delivered" and "read" indicators

2. **Typing Indicators**: Show when users are typing
   - Send typing events via WebSocket
   - Display "User is typing..." in UI

3. **File Attachments**: Support image/file sharing
   - Add `attachment_url` field to messages
   - Integrate with file storage (S3/Azure Blob)

4. **Message Search**: Full-text search across messages
   - Add SQLite FTS5 extension
   - Create search endpoint

5. **Notifications**: Push notifications for new messages
   - Integrate with push notification service
   - Store user notification preferences

### Performance
1. **Message Pagination**: Limit message history queries
   - Add `limit` and `offset` parameters
   - Implement infinite scroll in frontend

2. **Connection Pooling**: Reuse database connections
   - Already handled by `jmoiron/sqlx`

3. **Message Caching**: Cache recent messages
   - Use Redis for frequently accessed sessions
   - Reduce database load

### Database Improvements
1. **Add Indexes**: Already included in migration
   - Index on `chat_session_id` for fast message lookup
   - Index on `parties` for session search
   - Index on `created_at` for chronological ordering

2. **Soft Delete**: Track deleted sessions
   - Add `deleted_at` timestamp
   - Filter out deleted sessions in queries

3. **Unread Count**: Track unread messages per user
   - Add `last_read_at` to `chat_sessions` junction table
   - Calculate unread count in queries

### Observability
1. **Metrics**: Track WebSocket connections and messages
   - Number of active connections
   - Messages sent/received per second
   - Session creation rate

2. **Logging**: Structured logs for debugging
   - Log WebSocket connect/disconnect
   - Log message routing
   - Log errors with context

---

## 14. Example API Usage

### Create New Chat Session
```bash
POST /api/v1/chats
Authorization: Bearer <jwt_token>

{
  "parties": ["user-uuid-1", "user-uuid-2"]
}

Response:
{
  "success": true,
  "data": {
    "id": "session-uuid",
    "parties": "user-uuid-1,user-uuid-2",
    "status": "active",
    "created_at": "2026-01-10T12:00:00Z"
  }
}
```

### Get All Sessions
```bash
GET /api/v1/chats
Authorization: Bearer <jwt_token>

Response:
{
  "success": true,
  "data": [
    {
      "id": "session-uuid-1",
      "parties": "user-1,user-2",
      "status": "active",
      "created_at": "2026-01-10T12:00:00Z"
    }
  ]
}
```

### Get Message History
```bash
GET /api/v1/chats/{session_id}/messages?limit=50
Authorization: Bearer <jwt_token>

Response:
{
  "success": true,
  "data": [
    {
      "id": "msg-uuid-1",
      "chat_session_id": "session-uuid",
      "user_id": "user-1",
      "username": "john_doe",
      "message": "Hello!",
      "created_at": "2026-01-10T12:01:00Z"
    }
  ]
}
```

### WebSocket Connection
```javascript
// Connect to WebSocket
const ws = new WebSocket('ws://localhost:8080/api/v1/chats/session-uuid/ws');

// Send message
ws.send(JSON.stringify({
  type: 'message',
  payload: {
    message: 'Hello from client!'
  }
}));

// Receive message
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('New message:', data);
};
```

---

## Summary

This plan follows the existing project patterns and provides a complete, production-ready chat application with:

- ✅ Database schema with proper relationships
- ✅ Repository pattern for data access
- ✅ Service layer for business logic
- ✅ REST API for session management
- ✅ WebSocket for real-time messaging
- ✅ JWT authentication throughout
- ✅ Comprehensive testing strategy
- ✅ Frontend integration guidelines
- ✅ Security and performance recommendations

**Estimated Implementation Time:**
- Phase 1-2: 2-3 days (Backend REST API)
- Phase 3-4: 2-3 days (WebSocket + Integration)
- Phase 5: 2-3 days (Frontend)
- Phase 6: 1-2 days (Testing & Polish)

**Total: ~7-11 days** for a full-featured chat application.
