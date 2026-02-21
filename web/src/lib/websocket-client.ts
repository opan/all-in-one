import type {
	WebSocketMessage,
	MessagePayload,
	TypingPayload,
	ParticipantPayload,
	ErrorPayload,
	MessageHandler,
	TypingHandler,
	ParticipantHandler,
	ErrorHandler,
	StateChangeHandler,
} from "./websocket-types";
import { WebSocketState } from "./websocket-types";

export class ChatWebSocketClient {
	private ws: WebSocket | null = null;
	private sessionId: string;
	private reconnectAttempts = 0;
	private maxReconnectAttempts = 5;
	private reconnectDelay = 1000; // Start with 1 second
	private reconnectTimeout: number | null = null;
	private pingInterval: number | null = null;
	private state: WebSocketState = WebSocketState.DISCONNECTED;
	
	// Event handlers
	private messageHandlers: Set<MessageHandler> = new Set();
	private typingHandlers: Set<TypingHandler> = new Set();
	private joinHandlers: Set<ParticipantHandler> = new Set();
	private leaveHandlers: Set<ParticipantHandler> = new Set();
	private errorHandlers: Set<ErrorHandler> = new Set();
	private stateChangeHandlers: Set<StateChangeHandler> = new Set();

	constructor(sessionId: string) {
		this.sessionId = sessionId;
	}

	/**
	 * Connect to the WebSocket server
	 */
	public connect(): void {
		if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
			console.log("WebSocket already connected or connecting");
			return;
		}

		this.updateState(WebSocketState.CONNECTING);

		// Determine WebSocket URL (ws:// or wss://)
		const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
		const host = window.location.host;
		
		// Get JWT token from cookie
		const token = this.getCookie("access_token");
		const tokenParam = token ? `?token=${encodeURIComponent(token)}` : "";
		
		const wsUrl = `${protocol}//${host}/api/v1/chats/${this.sessionId}/ws${tokenParam}`;

		console.log("Connecting to WebSocket:", wsUrl.replace(/token=[^&]+/, "token=***"));

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
	public sendMessage(message: string): void {
		if (!this.isConnected()) {
			console.error("Cannot send message: WebSocket not connected");
			return;
		}

		const wsMessage: WebSocketMessage = {
			type: "message",
			payload: { message },
			timestamp: new Date().toISOString(),
		};

		this.send(wsMessage);
	}

	/**
	 * Send a typing indicator
	 */
	public sendTyping(isTyping: boolean): void {
		if (!this.isConnected()) {
			return;
		}

		const wsMessage: WebSocketMessage = {
			type: "typing",
			payload: { is_typing: isTyping },
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
			this.updateState(WebSocketState.DISCONNECTED);
			
			// Attempt reconnect if it wasn't a clean close
			if (event.code !== 1000 && event.code !== 1001) {
				console.warn("Unexpected close, attempting reconnect...");
				this.attemptReconnect();
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
				// Send a ping message (you can customize this based on your backend)
				const pingMessage: WebSocketMessage = {
					type: "typing",
					payload: { is_typing: false },
					timestamp: new Date().toISOString(),
				};
				this.send(pingMessage);
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
