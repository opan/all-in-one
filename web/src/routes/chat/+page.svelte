<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { Button } from "$lib/components/ui/button/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Switch } from "$lib/components/ui/switch/index";
  import * as Card from "$lib/components/ui/card/index";
  import { Search, Send, Users, MoreVertical, Plus, Bell } from "@lucide/svelte/icons";
  import { Separator } from "$lib/components/ui/separator/index";
  import {
    getSessions,
    getMessages,
    sendMessage as sendMessageApi,
    getReceivedInvites,
    respondToInvite,
    type ChatSession as ApiChatSession,
    type ChatMessage as ApiChatMessage,
    type ChatInvite,
  } from "$lib/chat-api";
  import { ChatWebSocketClient } from "$lib/websocket-client";
  import { WebSocketState } from "$lib/websocket-types";
  import type { MessagePayload, TypingPayload, InvitePayload } from "$lib/websocket-types";
  import NewChatDialog from "$components/NewChatDialog.svelte";

  // State
  let chatSessions = $state<ApiChatSession[]>([]);
  let messages = $state<ApiChatMessage[]>([]);
  let activeSessionId = $state<string | null>(null);
  let searchQuery = $state("");
  let newMessage = $state("");
  let loading = $state(true);
  let sessionsError = $state<string | null>(null); // Error loading sessions list
  let messagesError = $state<string | null>(null); // Error loading messages
  let currentUserId = $state<string | null>(null);
  let wsClient = $state<ChatWebSocketClient | null>(null);
  let wsState = $state<WebSocketState>(WebSocketState.DISCONNECTED);
  let typingUsers = $state<Set<string>>(new Set());
  let typingTimeout: number | null = null;
  let showNewChatDialog = $state(false);
  let messagesContainer: HTMLDivElement | null = $state(null);
  let autoScrollOnReceive = $state(true);

  // Invite state
  let receivedInvites = $state<ChatInvite[]>([]);
  let showInviteInbox = $state(false);
  let inviteActionLoading = $state<string | null>(null);

  // Load sessions on mount
  onMount(async () => {
    try {
      loading = true;
      sessionsError = null;
      messagesError = null;

      try {
        const stored = localStorage.getItem("chat-auto-scroll-on-receive");
        if (stored !== null) autoScrollOnReceive = stored === "true";
      } catch {
        // localStorage unavailable (e.g. private browsing) — keep default
      }

      // Get current user
      const userResponse = await fetch('/api/v1/users/me', { credentials: 'include' });
      if (userResponse.ok) {
        const userData = await userResponse.json();
        if (userData.success && userData.data) {
          currentUserId = userData.data.id;
        }
      }
      
      chatSessions = await getSessions(20, 0); // Load first 20 sessions
      
      // Load pending invites
      receivedInvites = await getReceivedInvites();
      
      // Connect WebSocket once (user-level connection)
      connectWebSocket();
      
      // Select first session by default if available
      if (chatSessions.length > 0) {
        activeSessionId = chatSessions[0].id;
        await loadMessages(chatSessions[0].id);
      }
    } catch (err) {
      sessionsError = err instanceof Error ? err.message : "Failed to load chats";
      console.error("Error loading chats:", err);
    } finally {
      loading = false;
    }
  });

  // Cleanup on component destroy
  onDestroy(() => {
    console.log("Chat component unmounting, disconnecting WebSocket...");
    disconnectWebSocket();
  });

  async function handleSessionCreated() {
    // Reload sessions after creating a new one
    try {
      sessionsError = null; // Clear any previous errors
      messagesError = null;
      chatSessions = await getSessions(20, 0); // Reload first 20 sessions
      
      // Select the newly created session (it should be first)
      if (chatSessions.length > 0) {
        const newSession = chatSessions[0];
        activeSessionId = newSession.id;
        await loadMessages(newSession.id);
      }
    } catch (err) {
      console.error("Error reloading sessions:", err);
      sessionsError = err instanceof Error ? err.message : "Failed to reload sessions";
    }
  }

  function connectWebSocket() {
    // Don't create a new connection if already connected
    if (wsClient && wsClient.isConnected()) {
      console.log("WebSocket already connected, skipping...");
      return;
    }

    // Disconnect any existing client first
    if (wsClient) {
      console.log("Disconnecting existing WebSocket before reconnecting...");
      wsClient.disconnect();
    }

    // Create user-level WebSocket client (receives messages from all sessions)
    wsClient = new ChatWebSocketClient();

    // Set up event handlers
    wsClient.onMessage((payload: MessagePayload) => {
      handleIncomingMessage(payload);
    });

    wsClient.onTyping((payload: TypingPayload) => {
      handleTypingIndicator(payload);
    });

    wsClient.onError((errorMsg: string) => {
      console.error("WebSocket error:", errorMsg);
      messagesError = errorMsg;
    });

    wsClient.onInvite((payload: InvitePayload) => {
      handleInviteEvent(payload);
    });

    wsClient.onStateChange((state: WebSocketState) => {
      wsState = state;
      console.log("WebSocket state changed:", state);
    });

    // Connect
    wsClient.connect();
  }

  function disconnectWebSocket() {
    if (wsClient) {
      wsClient.disconnect();
      wsClient = null;
    }
    wsState = WebSocketState.DISCONNECTED;
  }

  function handleIncomingMessage(payload: MessagePayload) {
    // Convert WebSocket payload to ChatMessage format
    const newMsg: ApiChatMessage = {
      id: payload.id,
      chat_session_id: payload.chat_session_id,
      user_id: payload.user_id,
      username: payload.username,
      message: payload.message,
      created_at: payload.created_at,
      sent_at: payload.created_at,
    };

    // If message is for the active session, add to messages list
    if (payload.chat_session_id === activeSessionId) {
      const exists = messages.some((m) => m.id === newMsg.id);
      if (!exists) {
        messages = [...messages, newMsg];

        // Messages sent by the current user (echoed back via the WebSocket
        // broadcast) always scroll into view; messages from others only
        // scroll when the user has opted in via the toggle.
        const isOwnMessage = payload.user_id === currentUserId;
        if (isOwnMessage || autoScrollOnReceive) {
          scrollToBottom();
        }
      }
    }

    // Always update last message in session list (this is the key improvement!)
    const session = chatSessions.find((s) => s.id === payload.chat_session_id);
    if (session) {
      session.last_message = newMsg;
      session.updated_at = newMsg.created_at;
      
      // Re-sort sessions by updated_at to bring this to the top
      chatSessions = chatSessions.sort((a, b) => 
        new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      );
    }
  }

  function handleTypingIndicator(payload: TypingPayload) {
    // Don't show typing indicator for current user
    if (payload.user_id === currentUserId) {
      return;
    }

    if (payload.is_typing) {
      typingUsers.add(payload.username);
    } else {
      typingUsers.delete(payload.username);
    }
    typingUsers = new Set(typingUsers); // Trigger reactivity
  }

  function handleInviteEvent(payload: InvitePayload) {
    if (payload.status === 'pending') {
      // New invite received — add to inbox if not already present
      const exists = receivedInvites.some(i => i.id === payload.invite_id);
      if (!exists) {
        const newInvite: ChatInvite = {
          id: payload.invite_id,
          batch_id: payload.batch_id,
          inviter_id: payload.inviter_id,
          invitee_id: payload.invitee_id,
          session_id: payload.session_id,
          status: 'pending',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          inviter_username: payload.inviter_username,
          invitee_username: payload.invitee_username,
        };
        receivedInvites = [newInvite, ...receivedInvites];
      }
    } else if (payload.status === 'accepted') {
      // An invite we sent was accepted and a session was created
      if (payload.session_id) {
        getSessions(20, 0).then(sessions => {
          chatSessions = sessions;
          // Navigate to the new session
          if (payload.session_id) {
            selectSession(payload.session_id);
          }
        }).catch(console.error);
      }
      // Remove from received inbox if present
      receivedInvites = receivedInvites.filter(i => i.id !== payload.invite_id);
    } else {
      // declined or cancelled — remove from inbox
      receivedInvites = receivedInvites.filter(i => i.id !== payload.invite_id);
    }
  }

  async function handleAcceptInvite(invite: ChatInvite) {
    inviteActionLoading = invite.id;
    try {
      const result = await respondToInvite(invite.id, 'accept');
      receivedInvites = receivedInvites.filter(i => i.id !== invite.id);
      if (result.session) {
        chatSessions = await getSessions(20, 0);
        await selectSession(result.session.id);
        showInviteInbox = false;
      }
    } catch (err) {
      console.error('Failed to accept invite:', err);
    } finally {
      inviteActionLoading = null;
    }
  }

  async function handleDeclineInvite(invite: ChatInvite) {
    inviteActionLoading = invite.id;
    try {
      await respondToInvite(invite.id, 'decline');
      receivedInvites = receivedInvites.filter(i => i.id !== invite.id);
    } catch (err) {
      console.error('Failed to decline invite:', err);
    } finally {
      inviteActionLoading = null;
    }
  }

  async function loadMessages(sessionId: string) {
    try {
      messages = await getMessages(sessionId);
      messagesError = null; // Clear error on successful load
      await scrollToBottom();
    } catch (err) {
      console.error("Error loading messages:", err);
      messagesError = err instanceof Error ? err.message : "Failed to load messages";
    }
  }

  // Scrolls the messages pane to the latest message. Always called after the
  // current user sends a message; called after receiving one only when
  // autoScrollOnReceive is on (toggle in the message input area).
  async function scrollToBottom() {
    await tick();
    if (messagesContainer) {
      messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
  }

  function toggleAutoScrollOnReceive(value: boolean) {
    autoScrollOnReceive = value;
    try {
      localStorage.setItem("chat-auto-scroll-on-receive", String(value));
    } catch {
      // localStorage unavailable (e.g. private browsing) — preference just won't persist
    }
  }

  // Computed values
  let filteredSessions = $derived(
    searchQuery.trim()
      ? chatSessions.filter((session) => {
          const participantNames = session.participants
            ?.map((p) => p.username || "")
            .join(" ");
          const searchLower = searchQuery.toLowerCase();
          return (
            (participantNames && participantNames.toLowerCase().includes(searchLower))
          );
        })
      : chatSessions
  );

  let activeSession = $derived(
    chatSessions.find((s) => s.id === activeSessionId)
  );

  let sessionMessages = $derived(
    activeSessionId ? messages.filter((m) => m.chat_session_id === activeSessionId) : []
  );

  async function selectSession(sessionId: string) {
    messagesError = null; // Clear any previous errors
    activeSessionId = sessionId;
    await loadMessages(sessionId);
  }

  async function sendMessage() {
    if (!newMessage.trim() || !activeSessionId) return;

    const messageText = newMessage;
    newMessage = ""; // Clear input immediately for better UX

    try {
      // Send via WebSocket if connected
      if (wsClient && wsClient.isConnected()) {
        wsClient.sendMessage(messageText, activeSessionId);
      } else {
        // Fallback to REST API if WebSocket is not connected
        console.warn("WebSocket not connected, using REST API fallback");
        const message = await sendMessageApi(activeSessionId, {
          message: messageText,
        });

        // Add message to local state
        messages = [...messages, message];
        await scrollToBottom();

        // Update last message in session
        const session = chatSessions.find((s) => s.id === activeSessionId);
        if (session) {
          session.last_message = message;
          session.updated_at = message.created_at;
        }
      }
    } catch (err) {
      console.error("Error sending message:", err);
      messagesError = err instanceof Error ? err.message : "Failed to send message";
      newMessage = messageText; // Restore message on error
    }
  }

  function handleInput() {
    // Send typing indicator when user starts typing
    if (wsClient && wsClient.isConnected() && activeSessionId) {
      wsClient.sendTyping(true, activeSessionId);
      
      // Clear existing timeout
      if (typingTimeout !== null) {
        clearTimeout(typingTimeout);
      }
      
      // Send "stopped typing" after 2 seconds of inactivity
      typingTimeout = window.setTimeout(() => {
        if (wsClient && wsClient.isConnected() && activeSessionId) {
          wsClient.sendTyping(false, activeSessionId);
        }
      }, 2000);
    }
  }

  function handleKeyPress(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }

  function formatTime(timestamp: string): string {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins} min ago`;
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
    if (diffDays === 1) return "Yesterday";
    if (diffDays < 7) return `${diffDays} days ago`;
    
    return date.toLocaleDateString();
  }

  function formatMessageTime(timestamp: string): string {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "2-digit",
    });
  }

  function getParticipantNames(session: ApiChatSession): string {
    if (!session.participants) return "";
    return session.participants
      .filter((p) => p.left_at === null && p.user_id !== currentUserId) // Only active participants, exclude current user
      .map((p) => p.username || `User ${p.user_id}`)
      .join(", ");
  }

  function getSessionName(session: ApiChatSession): string {
    if (!session.participants) return "Chat";
    
    const activeParticipants = session.participants.filter((p) => p.left_at === null);
    const otherParticipants = activeParticipants.filter((p) => p.user_id !== currentUserId);
    
    if (otherParticipants.length === 1) {
      // 1-on-1 chat: show the other user's name
      return otherParticipants[0]?.username || "Chat";
    }
    
    // Group chat: show participant count (excluding current user)
    return `Group (${otherParticipants.length})`;
  }
</script>

<!-- Height is viewport minus the app shell's h-14 header (app-layout.svelte); using
     h-screen here overflows the shell and pushes the message input below the fold. -->
<div class="flex h-[calc(100dvh-3.5rem)] bg-background">
  <!-- Left Panel: Chat Sessions List -->
  <div class="w-full md:w-80 border-r flex flex-col min-h-0 {activeSessionId ? 'hidden md:flex' : 'flex'}">
    <!-- Header -->
    <div class="p-4 border-b">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-lg font-semibold">Chats</h2>
        <div class="flex items-center gap-1">
          <!-- Invite inbox bell -->
          <Button
            variant="ghost"
            size="icon"
            class="h-8 w-8 relative"
            onclick={() => showInviteInbox = !showInviteInbox}
            aria-label="Invite inbox"
          >
            <Bell class="h-4 w-4" />
            {#if receivedInvites.length > 0}
              <span class="absolute -top-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-destructive text-[10px] font-bold text-destructive-foreground">
                {receivedInvites.length}
              </span>
            {/if}
          </Button>
          <Button variant="ghost" size="icon" class="h-8 w-8" onclick={() => showNewChatDialog = true}>
            <Plus class="h-4 w-4" />
          </Button>
        </div>
      </div>
      
      <!-- Search -->
      <div class="relative">
        <Search class="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          type="text"
          placeholder="Search chats..."
          class="pl-8"
          bind:value={searchQuery}
        />
      </div>
    </div>

    <!-- Invite Inbox (collapsible) -->
    {#if showInviteInbox}
      <div class="border-b bg-muted/30">
        <div class="px-4 py-2 flex items-center justify-between">
          <span class="text-sm font-medium">Pending Invites</span>
          <button class="text-xs text-muted-foreground hover:text-foreground" onclick={() => showInviteInbox = false}>
            Close
          </button>
        </div>
        {#if receivedInvites.length === 0}
          <div class="px-4 py-3 text-xs text-muted-foreground">No pending invites</div>
        {:else}
          <div class="max-h-52 overflow-y-auto divide-y">
            {#each receivedInvites as invite (invite.id)}
              <div class="px-4 py-3 space-y-2">
                <div class="text-sm">
                  <span class="font-medium">{invite.inviter_username ?? invite.inviter_id}</span>
                  {invite.session_id ? ' invited you to join a chat' : ' wants to chat with you'}
                </div>
                <div class="flex gap-2">
                  <Button
                    size="sm"
                    class="h-7 text-xs flex-1"
                    disabled={inviteActionLoading === invite.id}
                    onclick={() => handleAcceptInvite(invite)}
                  >
                    {inviteActionLoading === invite.id ? '...' : 'Accept'}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    class="h-7 text-xs flex-1"
                    disabled={inviteActionLoading === invite.id}
                    onclick={() => handleDeclineInvite(invite)}
                  >
                    Decline
                  </Button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Chat Sessions List -->
    <div class="flex-1 min-h-0 overflow-y-auto">
      {#if loading}
        <div class="p-4 text-center text-muted-foreground">
          <p>Loading chats...</p>
        </div>
      {:else if sessionsError}
        <div class="p-4 text-center text-destructive">
          <p class="text-sm">{sessionsError}</p>
        </div>
      {:else if filteredSessions.length === 0}
        <div class="p-4 text-center text-muted-foreground">
          <p>No chats found</p>
        </div>
      {:else}
        {#each filteredSessions as session (session.id)}
          <button
            class="w-full p-4 text-left hover:bg-accent transition-colors border-b {activeSessionId === session.id ? 'bg-accent' : ''}"
            onclick={() => selectSession(session.id)}
          >
            <div class="flex items-start justify-between mb-1">
              <h3 class="font-medium text-sm truncate flex-1">
                {getSessionName(session)}
              </h3>
            </div>
            <div class="flex items-center text-xs text-muted-foreground mb-1">
              <Users class="h-3 w-3 mr-1" />
              <span class="truncate">{getParticipantNames(session)}</span>
            </div>
            <div class="flex items-center justify-between">
              <p class="text-xs text-muted-foreground truncate flex-1">
                {session.last_message?.message || "No messages yet"}
              </p>
              <span class="text-xs text-muted-foreground ml-2 whitespace-nowrap">
                {session.last_message ? formatTime(session.last_message.created_at) : ""}
              </span>
            </div>
          </button>
        {/each}
      {/if}
    </div>
  </div>

  <!-- Right Panel: Chat Conversation -->
  <div class="flex-1 flex-col min-h-0 {activeSessionId ? 'flex' : 'hidden md:flex'}">
    {#if activeSession}
      <!-- Chat Header -->
      <div class="p-4 border-b flex items-center justify-between">
        <div class="flex items-center gap-2 min-w-0">
          <button
            class="md:hidden text-muted-foreground hover:text-foreground shrink-0"
            onclick={() => activeSessionId = null}
            aria-label="Back to chats"
          >
            ←
          </button>
          <div class="min-w-0">
            <h2 class="text-lg font-semibold truncate">{getSessionName(activeSession)}</h2>
            <div class="flex items-center text-sm text-muted-foreground">
              <Users class="h-3 w-3 mr-1 shrink-0" />
              <span class="truncate">{getParticipantNames(activeSession)}</span>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-3 shrink-0">
          <label class="hidden sm:flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
            <span>Auto-scroll on new messages</span>
            <Switch
              checked={autoScrollOnReceive}
              onCheckedChange={toggleAutoScrollOnReceive}
            />
          </label>
          <Button variant="ghost" size="icon" class="h-8 w-8 shrink-0">
            <MoreVertical class="h-4 w-4" />
          </Button>
        </div>
      </div>

      <!-- Messages Area -->
      <div class="flex-1 min-h-0 overflow-y-auto p-4 space-y-4" bind:this={messagesContainer}>
        <!-- Error Message -->
        {#if messagesError}
          <div class="p-3 bg-destructive/10 border border-destructive/20 rounded-lg">
            <p class="text-sm text-destructive">{messagesError}</p>
          </div>
        {/if}

        {#each sessionMessages as message (message.id)}
          {@const isCurrentUser = message.user_id === currentUserId}
          <div
            class="flex {isCurrentUser ? 'justify-end' : 'justify-start'}"
          >
            <div
              class="max-w-[70%] {isCurrentUser ? 'order-2' : 'order-1'}"
            >
              {#if !isCurrentUser}
                <p class="text-xs font-medium text-muted-foreground mb-1">
                  {message.username || `User ${message.user_id}`}
                </p>
              {/if}
              <div
                class="rounded-lg px-4 py-2 {isCurrentUser
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted'}"
              >
                <p class="text-sm">{message.message}</p>
              </div>
              <p class="text-xs text-muted-foreground mt-1 {isCurrentUser ? 'text-right' : 'text-left'}">
                {formatMessageTime(message.created_at)}
              </p>
            </div>
          </div>
        {/each}

        <!-- Typing Indicator -->
        {#if typingUsers.size > 0}
          <div class="flex justify-start">
            <div class="bg-muted rounded-lg px-4 py-2">
              <p class="text-xs text-muted-foreground italic">
                {Array.from(typingUsers).join(", ")} {typingUsers.size === 1 ? "is" : "are"} typing...
              </p>
            </div>
          </div>
        {/if}
      </div>

      <!-- Message Input -->
      <div class="p-4 border-t">
        <!-- Connection Status -->
        {#if wsState === WebSocketState.REPLACED}
          <div class="mb-2 p-2 bg-yellow-500/10 border border-yellow-500/20 rounded-lg text-xs flex items-center justify-between gap-2">
            <span class="text-yellow-600">
              This chat is open in another tab or window — live updates are paused here.
            </span>
            <Button size="sm" variant="outline" class="h-6 text-xs shrink-0" onclick={() => connectWebSocket()}>
              Use this tab
            </Button>
          </div>
        {:else if wsState !== WebSocketState.CONNECTED}
          <div class="mb-2 text-xs text-center">
            <span class="text-yellow-600">
              {#if wsState === WebSocketState.CONNECTING}
                Connecting...
              {:else if wsState === WebSocketState.DISCONNECTED}
                Disconnected - messages will be sent via fallback
              {:else if wsState === WebSocketState.ERROR}
                Connection error - using fallback mode
              {/if}
            </span>
          </div>
        {/if}
        
        <div class="flex gap-2">
          <Input
            type="text"
            placeholder="Type a message..."
            class="flex-1"
            bind:value={newMessage}
            oninput={handleInput}
            onkeypress={handleKeyPress}
          />
          <Button onclick={sendMessage} disabled={!newMessage.trim()}>
            <Send class="h-4 w-4" />
          </Button>
        </div>
      </div>
    {:else if loading}
      <div class="flex-1 flex items-center justify-center text-muted-foreground">
        <div class="text-center">
          <p>Loading...</p>
        </div>
      </div>
    {:else}
      <!-- No Chat Selected -->
      <div class="flex-1 flex items-center justify-center text-muted-foreground">
        <div class="text-center">
          <Users class="h-12 w-12 mx-auto mb-4 opacity-50" />
          <p class="text-lg font-medium mb-1">Select a chat to start messaging</p>
          <p class="text-sm">Choose a conversation from the list</p>
        </div>
      </div>
    {/if}
  </div>
</div>

<!-- New Chat Dialog -->
<NewChatDialog 
  bind:open={showNewChatDialog} 
  onSessionCreated={handleSessionCreated}
/>
