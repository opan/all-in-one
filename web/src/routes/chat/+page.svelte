<script lang="ts">
  import { Button } from "$lib/components/ui/button/index";
  import { Input } from "$lib/components/ui/input/index";
  import * as Card from "$lib/components/ui/card/index";
  import { Search, Send, Users, MoreVertical, Plus } from "@lucide/svelte/icons";
  import { Separator } from "$lib/components/ui/separator/index";

  interface ChatSession {
    id: number;
    name: string;
    participants: string[];
    lastMessage: string;
    lastMessageTime: string;
    unreadCount: number;
  }

  interface Message {
    id: number;
    sessionId: number;
    sender: string;
    content: string;
    timestamp: string;
    isCurrentUser: boolean;
  }

  // Mock data for chat sessions
  let chatSessions = $state<ChatSession[]>([
    {
      id: 1,
      name: "Project Discussion",
      participants: ["Alice", "Bob", "Charlie"],
      lastMessage: "Let's meet tomorrow at 10 AM",
      lastMessageTime: "2 min ago",
      unreadCount: 3,
    },
    {
      id: 2,
      name: "Design Team",
      participants: ["Diana", "Eve"],
      lastMessage: "I've uploaded the new mockups",
      lastMessageTime: "1 hour ago",
      unreadCount: 0,
    },
    {
      id: 3,
      name: "Book Club",
      participants: ["Frank", "Grace", "Henry", "Iris"],
      lastMessage: "What do you think about the last chapter?",
      lastMessageTime: "Yesterday",
      unreadCount: 5,
    },
  ]);

  // Mock data for messages
  let messages = $state<Message[]>([
    {
      id: 1,
      sessionId: 1,
      sender: "Alice",
      content: "Hey everyone! How's the project coming along?",
      timestamp: "10:30 AM",
      isCurrentUser: false,
    },
    {
      id: 2,
      sessionId: 1,
      sender: "You",
      content: "Going well! I just finished the backend API.",
      timestamp: "10:32 AM",
      isCurrentUser: true,
    },
    {
      id: 3,
      sessionId: 1,
      sender: "Bob",
      content: "Great! I'm working on the frontend integration now.",
      timestamp: "10:35 AM",
      isCurrentUser: false,
    },
    {
      id: 4,
      sessionId: 1,
      sender: "Charlie",
      content: "Perfect timing! Can we sync up tomorrow?",
      timestamp: "10:40 AM",
      isCurrentUser: false,
    },
    {
      id: 5,
      sessionId: 1,
      sender: "Alice",
      content: "Let's meet tomorrow at 10 AM",
      timestamp: "10:42 AM",
      isCurrentUser: false,
    },
  ]);

  let activeSessionId = $state<number | null>(1);
  let searchQuery = $state("");
  let newMessage = $state("");

  // Computed values
  let filteredSessions = $derived(
    searchQuery.trim()
      ? chatSessions.filter(
          (session) =>
            session.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            session.participants.some((p) =>
              p.toLowerCase().includes(searchQuery.toLowerCase())
            )
        )
      : chatSessions
  );

  let activeSession = $derived(
    chatSessions.find((s) => s.id === activeSessionId)
  );

  let sessionMessages = $derived(
    activeSessionId
      ? messages.filter((m) => m.sessionId === activeSessionId)
      : []
  );

  function selectSession(sessionId: number) {
    activeSessionId = sessionId;
    // Mark as read
    const session = chatSessions.find((s) => s.id === sessionId);
    if (session) {
      session.unreadCount = 0;
    }
  }

  function sendMessage() {
    if (!newMessage.trim() || !activeSessionId) return;

    const message: Message = {
      id: messages.length + 1,
      sessionId: activeSessionId,
      sender: "You",
      content: newMessage,
      timestamp: new Date().toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
      }),
      isCurrentUser: true,
    };

    messages = [...messages, message];
    
    // Update last message in session
    const session = chatSessions.find((s) => s.id === activeSessionId);
    if (session) {
      session.lastMessage = newMessage;
      session.lastMessageTime = "Just now";
    }

    newMessage = "";
  }

  function handleKeyPress(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
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
      {#if filteredSessions.length === 0}
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
                {session.name}
              </h3>
              {#if session.unreadCount > 0}
                <span class="ml-2 bg-primary text-primary-foreground text-xs rounded-full px-2 py-0.5 font-medium">
                  {session.unreadCount}
                </span>
              {/if}
            </div>
            <div class="flex items-center text-xs text-muted-foreground mb-1">
              <Users class="h-3 w-3 mr-1" />
              <span class="truncate">{session.participants.join(", ")}</span>
            </div>
            <div class="flex items-center justify-between">
              <p class="text-xs text-muted-foreground truncate flex-1">
                {session.lastMessage}
              </p>
              <span class="text-xs text-muted-foreground ml-2 whitespace-nowrap">
                {session.lastMessageTime}
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
          <h2 class="text-lg font-semibold">{activeSession.name}</h2>
          <div class="flex items-center text-sm text-muted-foreground">
            <Users class="h-3 w-3 mr-1" />
            <span>{activeSession.participants.join(", ")}</span>
          </div>
        </div>
        <Button variant="ghost" size="icon" class="h-8 w-8">
          <MoreVertical class="h-4 w-4" />
        </Button>
      </div>

      <!-- Messages Area -->
      <div class="flex-1 overflow-y-auto p-4 space-y-4">
        {#each sessionMessages as message (message.id)}
          <div
            class="flex {message.isCurrentUser ? 'justify-end' : 'justify-start'}"
          >
            <div
              class="max-w-[70%] {message.isCurrentUser ? 'order-2' : 'order-1'}"
            >
              {#if !message.isCurrentUser}
                <p class="text-xs font-medium text-muted-foreground mb-1">
                  {message.sender}
                </p>
              {/if}
              <div
                class="rounded-lg px-4 py-2 {message.isCurrentUser
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted'}"
              >
                <p class="text-sm">{message.content}</p>
              </div>
              <p class="text-xs text-muted-foreground mt-1 {message.isCurrentUser ? 'text-right' : 'text-left'}">
                {message.timestamp}
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
