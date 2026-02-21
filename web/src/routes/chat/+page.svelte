<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button/index";
  import { Input } from "$lib/components/ui/input/index";
  import * as Card from "$lib/components/ui/card/index";
  import { Search, Send, Users, MoreVertical, Plus } from "@lucide/svelte/icons";
  import { Separator } from "$lib/components/ui/separator/index";
  import {
    getSessions,
    getMessages,
    sendMessage as sendMessageApi,
    type ChatSession as ApiChatSession,
    type ChatMessage as ApiChatMessage,
  } from "$lib/chat-api";

  // State
  let chatSessions = $state<ApiChatSession[]>([]);
  let messages = $state<ApiChatMessage[]>([]);
  let activeSessionId = $state<string | null>(null);
  let searchQuery = $state("");
  let newMessage = $state("");
  let loading = $state(true);
  let error = $state<string | null>(null);
  let currentUserId = $state<string | null>(null);

  // Load sessions on mount
  onMount(async () => {
    try {
      loading = true;
      error = null;
      
      // Get current user
      const userResponse = await fetch('/api/v1/users/me', { credentials: 'include' });
      if (userResponse.ok) {
        const userData = await userResponse.json();
        if (userData.success && userData.data) {
          currentUserId = userData.data.id;
        }
      }
      
      chatSessions = await getSessions();
      
      // Select first session by default if available
      if (chatSessions.length > 0) {
        activeSessionId = chatSessions[0].id;
        await loadMessages(chatSessions[0].id);
      }
    } catch (err) {
      error = err instanceof Error ? err.message : "Failed to load chats";
      console.error("Error loading chats:", err);
    } finally {
      loading = false;
    }
  });

  async function loadMessages(sessionId: string) {
    try {
      messages = await getMessages(sessionId);
    } catch (err) {
      console.error("Error loading messages:", err);
      error = err instanceof Error ? err.message : "Failed to load messages";
    }
  }

  // Computed values
  let filteredSessions = $derived(
    searchQuery.trim()
      ? chatSessions.filter((session) => {
          const sessionName = session.name || "";
          const participantNames = session.participants
            ?.map((p) => p.user?.username || "")
            .join(" ");
          const searchLower = searchQuery.toLowerCase();
          return (
            sessionName.toLowerCase().includes(searchLower) ||
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
    activeSessionId = sessionId;
    await loadMessages(sessionId);
  }

  async function sendMessage() {
    if (!newMessage.trim() || !activeSessionId) return;

    try {
      const message = await sendMessageApi(activeSessionId, {
        message: newMessage,
      });

      // Add message to local state
      messages = [...messages, message];

      // Update last message in session
      const session = chatSessions.find((s) => s.id === activeSessionId);
      if (session) {
        session.last_message = message;
        session.updated_at = message.created_at;
      }

      newMessage = "";
    } catch (err) {
      console.error("Error sending message:", err);
      error = err instanceof Error ? err.message : "Failed to send message";
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
      .filter((p) => p.left_at === null) // Only active participants
      .map((p) => p.username || `User ${p.user_id}`)
      .join(", ");
  }

  function getSessionName(session: ApiChatSession): string {
    if (!session.participants) return "Chat";
    
    const activeParticipants = session.participants.filter((p) => p.left_at === null);
    if (activeParticipants.length === 2) {
      // 1-on-1 chat: show the other user's name
      const otherUser = activeParticipants.find((p) => p.user_id !== currentUserId);
      return otherUser?.username || "Chat";
    }
    
    // Group chat: show participant count
    return `Group (${activeParticipants.length})`;
  }
</script>

<div class="flex h-screen bg-background">
  <!-- Left Panel: Chat Sessions List -->
  <div class="w-80 border-r flex flex-col">
    <!-- Header -->
    <div class="p-4 border-b">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-lg font-semibold">Chats</h2>
        <Button variant="ghost" size="icon" class="h-8 w-8">
          <Plus class="h-4 w-4" />
        </Button>
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

    <!-- Chat Sessions List -->
    <div class="flex-1 overflow-y-auto">
      {#if loading}
        <div class="p-4 text-center text-muted-foreground">
          <p>Loading chats...</p>
        </div>
      {:else if error}
        <div class="p-4 text-center text-destructive">
          <p class="text-sm">{error}</p>
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
  <div class="flex-1 flex flex-col">
    {#if activeSession}
      <!-- Chat Header -->
      <div class="p-4 border-b flex items-center justify-between">
        <div>
          <h2 class="text-lg font-semibold">{getSessionName(activeSession)}</h2>
          <div class="flex items-center text-sm text-muted-foreground">
            <Users class="h-3 w-3 mr-1" />
            <span>{getParticipantNames(activeSession)}</span>
          </div>
        </div>
        <Button variant="ghost" size="icon" class="h-8 w-8">
          <MoreVertical class="h-4 w-4" />
        </Button>
      </div>

      <!-- Messages Area -->
      <div class="flex-1 overflow-y-auto p-4 space-y-4">
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
      </div>

      <!-- Message Input -->
      <div class="p-4 border-t">
        <div class="flex gap-2">
          <Input
            type="text"
            placeholder="Type a message..."
            class="flex-1"
            bind:value={newMessage}
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
