import type {
	WebSocketMessage,
	MessagePayload,
	TypingPayload,
	ParticipantPayload,
	ErrorPayload,
	InvitePayload,
	MessageHandler,
	TypingHandler,
	ParticipantHandler,
	ErrorHandler,
	StateChangeHandler,
	InviteHandler,
} from "./websocket-types";
import { WebSocketState, REPLACED_CONNECTION_CLOSE_CODE } from "./websocket-types";

export class ChatWebSocketClient {
	private ws: WebSocket | null = null;
	private reconnectAttempts = 0;
	private maxReconnectAttempts = 5;
	private reconnectDelay = 1000; // Start with 1 second
	private reconnectTimeout: number | null = null;
	private pingInterval: number | null = null;
	private state: WebSocketState = WebSocketState.DISCONNECTED;
	private intentionalClose = false; // Flag to prevent reconnection on intentional disconnect
	
	// Event handlers
	private messageHandlers: Set<MessageHandler> = new Set();
	private typingHandlers: Set<TypingHandler> = new Set();
	private joinHandlers: Set<ParticipantHandler> = new Set();
	private leaveHandlers: Set<ParticipantHandler> = new Set();
	private errorHandlers: Set<ErrorHandler> = new Set();
	private stateChangeHandlers: Set<StateChangeHandler> = new Set();
	private inviteHandlers: Set<InviteHandler> = new Set();

	constructor() {
		// No sessionId needed - this is a user-level connection
	}

	/**
	 * Connect to the WebSocket server
	 */
	public connect(): void {
		if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
			console.log("WebSocket already connected or connecting, state:", this.ws.readyState);
			return;
		}

		// Reset intentional close flag
		this.intentionalClose = false;
		console.log("🔌 Connecting to WebSocket...");
		this.updateState(WebSocketState.CONNECTING);

		// Determine WebSocket URL (ws:// or wss://)
		const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
		
		// In development (Vite), connect directly to backend to avoid proxy issues
		// In production, use the same host as the frontend
		const isDev = import.meta.env.DEV;
		const host = isDev ? "localhost:8080" : window.location.host;
		
		// Get JWT token from cookie
		const token = this.getCookie("access_token");
		const tokenParam = token ? `?token=${encodeURIComponent(token)}` : "";
		
		const wsUrl = `${protocol}//${host}/api/v1/ws${tokenParam}`;

		console.log("Connecting to WebSocket:", wsUrl.replace(/token=[^&]+/, "token=***"), `(${isDev ? 'development' : 'production'} mode)`);

		try {
			this.ws = new WebSocket(wsUrl);
			this.setupEventListeners();
		} catch (error) {
			console.error("Failed to create WebSocket:", error);
			this.updateState(WebSocketState.ERROR);
			this.attemptReconnect();
		}
	}

	/**
	 * Disconnect from the WebSocket server
	 */
	public disconnect(): void {
		this.intentionalClose = true; // Mark as intentional
		this.updateState(WebSocketState.DISCONNECTING);
		this.clearReconnectTimeout();
		this.clearPingInterval();
		
		if (this.ws) {
			this.ws.close(1000, "Client disconnecting");
			this.ws = null;
		}
		
		this.updateState(WebSocketState.DISCONNECTED);
	}

	/**
	 * Send a message through the WebSocket
	 */
	public sendMessage(message: string, sessionId: string): void {
		if (!this.isConnected()) {
			console.error("Cannot send message: WebSocket not connected");
			return;
		}

		const wsMessage: WebSocketMessage = {
			type: "message",
			payload: { message, session_id: sessionId },
			timestamp: new Date().toISOString(),
		};

		this.send(wsMessage);
	}

	/**
	 * Send a typing indicator
	 */
	public sendTyping(isTyping: boolean, sessionId: string): void {
		if (!this.isConnected()) {
			return;
		}

		const wsMessage: WebSocketMessage = {
			type: "typing",
			payload: { is_typing: isTyping, session_id: sessionId },
			timestamp: new Date().toISOString(),
		};

		this.send(wsMessage);
	}

	/**
	 * Check if WebSocket is connected
	 */
	public isConnected(): boolean {
		return this.ws?.readyState === WebSocket.OPEN;
	}

	/**
	 * Get current connection state
	 */
	public getState(): WebSocketState {
		return this.state;
	}

	/**
	 * Register event handlers
	 */
	public onMessage(handler: MessageHandler): () => void {
		this.messageHandlers.add(handler);
		return () => this.messageHandlers.delete(handler);
	}

	public onTyping(handler: TypingHandler): () => void {
		this.typingHandlers.add(handler);
		return () => this.typingHandlers.delete(handler);
	}

	public onJoin(handler: ParticipantHandler): () => void {
		this.joinHandlers.add(handler);
		return () => this.joinHandlers.delete(handler);
	}

	public onLeave(handler: ParticipantHandler): () => void {
		this.leaveHandlers.add(handler);
		return () => this.leaveHandlers.delete(handler);
	}

	public onError(handler: ErrorHandler): () => void {
		this.errorHandlers.add(handler);
		return () => this.errorHandlers.delete(handler);
	}

	public onStateChange(handler: StateChangeHandler): () => void {
		this.stateChangeHandlers.add(handler);
		return () => this.stateChangeHandlers.delete(handler);
	}

	/**
	 * Register a handler for invite lifecycle events:
	 * invite_received, invite_accepted, invite_declined, invite_cancelled
	 */
	public onInvite(handler: InviteHandler): () => void {
		this.inviteHandlers.add(handler);
		return () => this.inviteHandlers.delete(handler);
	}

	/**
	 * Setup WebSocket event listeners
	 */
	private setupEventListeners(): void {
		if (!this.ws) return;

		this.ws.onopen = () => {
			console.log("✅ WebSocket connected successfully");
			this.reconnectAttempts = 0;
			this.reconnectDelay = 1000;
			this.updateState(WebSocketState.CONNECTED);
			this.startPingInterval();
		};

		this.ws.onclose = (event) => {
			console.log(`❌ WebSocket closed: code=${event.code}, reason="${event.reason || 'No reason provided'}"`);
			this.clearPingInterval();

			// The server replaced this connection with a newer one for the same user
			// (most commonly: this chat was opened in another browser tab). Do NOT
			// reconnect here — reconnecting would just kick out the other tab and
			// cause both tabs to fight over the connection indefinitely.
			if (event.code === REPLACED_CONNECTION_CLOSE_CODE) {
				this.intentionalClose = true;
				this.clearReconnectTimeout();
				this.updateState(WebSocketState.REPLACED);
				console.warn("WebSocket replaced by a newer connection for this user (likely open in another tab)");
				return;
			}

			this.updateState(WebSocketState.DISCONNECTED);

			// Only attempt reconnect if:
			// 1. It wasn't an intentional close
			// 2. It wasn't a clean close (1000 = normal, 1001 = going away)
			if (!this.intentionalClose && event.code !== 1000 && event.code !== 1001) {
				console.warn("Unexpected close, attempting reconnect...");
				this.attemptReconnect();
			} else {
				console.log("WebSocket closed cleanly, not reconnecting");
			}
		};

		this.ws.onerror = (error) => {
			console.error("❌ WebSocket error:", error);
			this.updateState(WebSocketState.ERROR);
		};

		this.ws.onmessage = (event) => {
			try {
				const wsMessage: WebSocketMessage = JSON.parse(event.data);
				this.handleMessage(wsMessage);
			} catch (error) {
				console.error("Failed to parse WebSocket message:", error);
			}
		};
	}

	/**
	 * Handle incoming WebSocket messages
	 */
	private handleMessage(wsMessage: WebSocketMessage): void {
		switch (wsMessage.type) {
			case "message": {
				const payload = wsMessage.payload as MessagePayload;
				this.messageHandlers.forEach((handler) => handler(payload));
				break;
			}
			case "typing": {
				const payload = wsMessage.payload as TypingPayload;
				this.typingHandlers.forEach((handler) => handler(payload));
				break;
			}
			case "join": {
				const payload = wsMessage.payload as ParticipantPayload;
				this.joinHandlers.forEach((handler) => handler(payload));
				break;
			}
			case "leave": {
				const payload = wsMessage.payload as ParticipantPayload;
				this.leaveHandlers.forEach((handler) => handler(payload));
				break;
			}
			case "error": {
				const payload = wsMessage.payload as ErrorPayload;
				this.errorHandlers.forEach((handler) => handler(payload.error));
				break;
			}
			case "invite_received":
			case "invite_accepted":
			case "invite_declined":
			case "invite_cancelled": {
				const payload = wsMessage.payload as InvitePayload;
				this.inviteHandlers.forEach((handler) => handler(payload));
				break;
			}
			default:
				console.warn("Unknown WebSocket message type:", wsMessage.type);
		}
	}

	/**
	 * Send a WebSocket message
	 */
	private send(message: WebSocketMessage): void {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
			console.error("Cannot send: WebSocket not open");
			return;
		}

		try {
			this.ws.send(JSON.stringify(message));
		} catch (error) {
			console.error("Failed to send WebSocket message:", error);
		}
	}

	/**
	 * Attempt to reconnect with exponential backoff
	 */
	private attemptReconnect(): void {
		if (this.reconnectAttempts >= this.maxReconnectAttempts) {
			console.error("Max reconnect attempts reached");
			this.updateState(WebSocketState.ERROR);
			return;
		}

		this.reconnectAttempts++;
		const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
		
		console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
		
		this.reconnectTimeout = window.setTimeout(() => {
			this.connect();
		}, delay);
	}

	/**
	 * Start ping interval to keep connection alive
	 */
	private startPingInterval(): void {
		this.clearPingInterval();
		
		// Send ping every 30 seconds
		this.pingInterval = window.setInterval(() => {
			if (this.isConnected()) {
				// WebSocket connection stays alive with native ping/pong
				// We don't need to send application-level pings
			}
		}, 30000);
	}

	/**
	 * Clear reconnect timeout
	 */
	private clearReconnectTimeout(): void {
		if (this.reconnectTimeout !== null) {
			clearTimeout(this.reconnectTimeout);
			this.reconnectTimeout = null;
		}
	}

	/**
	 * Clear ping interval
	 */
	private clearPingInterval(): void {
		if (this.pingInterval !== null) {
			clearInterval(this.pingInterval);
			this.pingInterval = null;
		}
	}

	/**
	 * Update connection state and notify listeners
	 */
	private updateState(newState: WebSocketState): void {
		if (this.state !== newState) {
			this.state = newState;
			this.stateChangeHandlers.forEach((handler) => handler(newState));
		}
	}

	/**
	 * Get cookie value by name
	 */
	private getCookie(name: string): string | null {
		const value = `; ${document.cookie}`;
		const parts = value.split(`; ${name}=`);
		if (parts.length === 2) {
			const cookieValue = parts.pop()?.split(';').shift();
			return cookieValue || null;
		}
		return null;
	}
}
