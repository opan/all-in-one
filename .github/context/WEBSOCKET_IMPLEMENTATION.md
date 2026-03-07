# WebSocket Implementation Summary

**Last Updated**: February 21, 2026  
**Status**: ✅ **COMPLETE AND WORKING**

## Overview

Real-time chat functionality using WebSocket connections for instant message delivery and typing indicators.

---

## Architecture

### Backend Components

1. **Hub** (`internal/chat/websocket/hub.go`)
   - Central connection manager
   - Maintains sessions → clients mapping
   - Handles register/unregister events
   - Broadcasts messages to session participants

2. **Client** (`internal/chat/websocket/client.go`)
   - Represents individual WebSocket connection
   - Read/write pumps for bi-directional communication
   - Message handler callback for processing incoming messages
   - **Independent context lifecycle** (not tied to HTTP request)

3. **Handler** (`internal/chat/handler/message.go`)
   - HTTP → WebSocket upgrade
   - JWT authentication via query parameter
   - Message processing and persistence
   - Integration with chat service

### Frontend Components

1. **Types** (`web/src/lib/websocket-types.ts`)
   - WebSocketMessage<T> interface
   - MessagePayload, TypingPayload, ErrorPayload
   - WebSocketState enum
   - Type-safe message handlers

2. **Client** (`web/src/lib/websocket-client.ts`)
   - ChatWebSocketClient class
   - Connection management with auto-reconnect
   - Exponential backoff for reconnection
   - Event-driven message handling
   - JWT token from cookie → query parameter

3. **UI Integration** (`web/src/routes/chat/+page.svelte`)
   - Real-time message receiving
   - Typing indicators
   - Connection status display
   - Fallback to REST API

---

## Issues Resolved

### Issue 1: WebSocket Authentication Failure
**Problem**: WebSocket stuck in "Connecting..." state, JWT cookies not sent with WebSocket upgrade

**Root Cause**: Native WebSocket API doesn't automatically send cookies

**Solution**:
- Modified JWT middleware to accept token from query parameter: `?token=<jwt>`
- Updated frontend to extract JWT from cookie and append to WebSocket URL
- File: `internal/authnz/middleware/jwt.go` - Extended validateJWT()

### Issue 2: Vite Proxy Not Forwarding WebSocket
**Problem**: WebSocket connections failing to reach backend

**Root Cause**: Vite proxy configuration missing WebSocket support

**Solution**:
- Added `ws: true` to proxy configuration
- File: `web/vite.config.ts`

```typescript
proxy: {
  '/api': {
    target: 'http://localhost:8080',
    changeOrigin: true,
    ws: true  // ← Critical for WebSocket
  }
}
```

### Issue 3: Messages Not Persisting
**Problem**: WebSocket connected but messages not saving to database

**Root Cause**: No message processing logic in WebSocket message handler

**Solution**:
- Added MessageHandler callback pattern to Client
- Implemented handleWebSocketMessage() to process and persist messages
- Files: `internal/chat/websocket/client.go`, `internal/chat/handler/message.go`

### Issue 4: Context Canceled Errors ⭐ CRITICAL
**Problem**: "context canceled" error when sending messages via WebSocket

**Root Cause**: Using HTTP request context for WebSocket connection. HTTP context is canceled after the upgrade completes, but WebSocket connections live much longer.

**Solution**:
- Created independent long-lived context for WebSocket connections
- Used `context.WithCancel(context.Background())` instead of `r.Context()`
- Copied request_id from HTTP context for logging continuity
- Added cancel function to Client struct for proper cleanup
- Cancel called in ReadPump defer when connection closes

**Code Changes**:
```go
// Create independent context for WebSocket (not tied to HTTP request)
wsCtx, wsCancel := context.WithCancel(context.Background())

// Copy important values from request context
if requestID := ctx.Value("request_id"); requestID != nil {
    wsCtx = context.WithValue(wsCtx, "request_id", requestID)
}

// Pass to client
client := websocket.NewClient(..., wsCtx, wsCancel, ...)
```

---

## Authentication Flow

1. User logs in → receives JWT in cookie
2. Frontend connects to WebSocket:
   - Extracts JWT from cookie using `getCookie()` helper
   - Appends to URL: `ws://localhost:5173/api/v1/chats/{sessionId}/ws?token={jwt}`
3. Vite proxy forwards to backend: `ws://localhost:8080/api/v1/chats/{sessionId}/ws?token={jwt}`
4. Backend JWT middleware validates token from query parameter
5. WebSocket upgrade succeeds

---

## Message Flow

### Sending Messages

1. User types message in UI
2. Frontend sends via WebSocket:
   ```typescript
   client.sendMessage(sessionId, messageText)
   ```
3. Backend Client.ReadPump() receives message
4. Calls messageHandler with message data
5. Handler creates message in database
6. Handler broadcasts to all session participants
7. All connected clients receive message instantly

### Receiving Messages

1. Backend broadcasts message to session
2. Frontend WebSocket onmessage event fires
3. Message handler processes based on type:
   - `"message"` → Add to message list
   - `"typing"` → Show typing indicator
   - `"error"` → Display error
4. UI updates reactively (Svelte runes)

---

## Connection Management

### Lifecycle

1. **Connect**: User navigates to chat session
2. **Authenticate**: JWT validated via query parameter
3. **Register**: Client added to Hub session map
4. **Active**: Bi-directional communication
5. **Disconnect**: User leaves or connection error
6. **Unregister**: Client removed from Hub
7. **Cleanup**: Context canceled, resources released

### Auto-Reconnect

- Exponential backoff: 1s, 2s, 4s, 8s, 16s (max)
- Automatic reconnection on connection loss
- Connection state tracking: CONNECTING, CONNECTED, DISCONNECTED, ERROR
- Visual status indicator in UI

### Keep-Alive

- Ping/pong mechanism (60s pong wait, 54s ping period)
- Detects dead connections
- Automatic cleanup of stale clients

---

## Key Files Modified

### Backend
- `internal/chat/websocket/hub.go` - Connection hub
- `internal/chat/websocket/client.go` - Client with context lifecycle
- `internal/chat/handler/message.go` - WebSocket upgrade & message processing
- `internal/authnz/middleware/jwt.go` - Query parameter auth
- `cmd/all-in-one/server/server.go` - Hub initialization

### Frontend
- `web/src/lib/websocket-types.ts` - TypeScript types
- `web/src/lib/websocket-client.ts` - Client class
- `web/src/routes/chat/+page.svelte` - UI integration
- `web/vite.config.ts` - Proxy configuration
- `web/.env` - API base URL

---

## Testing Checklist

- [x] WebSocket connection establishes successfully
- [x] JWT authentication works with query parameter
- [x] Messages send and persist to database
- [x] Messages broadcast to all session participants
- [x] Real-time updates appear instantly
- [x] Typing indicators work
- [x] Connection status displays correctly
- [x] Auto-reconnect works after disconnect
- [x] Messages persist after page refresh
- [x] Multiple users can chat simultaneously
- [x] No context cancellation errors
- [x] Proper cleanup on disconnect

---

## Performance Considerations

- **Message buffering**: 256 message buffer per client
- **Max message size**: 8KB
- **Connection limits**: No hard limit (OS dependent)
- **Memory**: ~64KB per active connection (buffers + overhead)
- **Database**: One INSERT per message, broadcasts don't hit DB

---

## Future Enhancements

1. **Read receipts**: Track when users read messages
2. **File attachments**: Support image/file uploads
3. **Presence indicators**: Show online/offline status
4. **Message editing**: Edit sent messages
5. **Message deletion**: Delete sent messages
6. **Search**: Full-text search across messages
7. **Emoji reactions**: React to messages
8. **Voice/video**: WebRTC integration

---

## Lessons Learned

1. **WebSocket context lifecycle**: Don't use HTTP request context for WebSocket connections
2. **Browser WebSocket API**: Doesn't send cookies automatically, use query parameters
3. **Vite proxy**: Requires explicit `ws: true` for WebSocket forwarding
4. **Type safety**: TypeScript + Go strong typing prevents many runtime errors
5. **Logging**: Request ID propagation crucial for debugging distributed systems
6. **Testing**: Real browser testing essential for WebSocket functionality

---

## References

- [gorilla/websocket documentation](https://github.com/gorilla/websocket)
- [MDN WebSocket API](https://developer.mozilla.org/en-US/docs/Web/API/WebSocket)
- [Vite proxy configuration](https://vitejs.dev/config/server-options.html#server-proxy)
