// WebSocket message types matching backend implementation
export type WebSocketMessageType = "message" | "join" | "leave" | "typing" | "error";

// WebSocket message envelope
export interface WebSocketMessage<T = unknown> {
	type: WebSocketMessageType;
	payload: T;
	timestamp: string;
}

// Message payload for 'message' type
export interface MessagePayload {
	id: string;
	chat_session_id: string;
	user_id: string;
	username: string;
	message: string;
	created_at: string;
}

// Typing indicator payload for 'typing' type
export interface TypingPayload {
	user_id: string;
	username: string;
	is_typing: boolean;
}

// Error payload for 'error' type
export interface ErrorPayload {
	error: string;
}

// Join/Leave payload
export interface ParticipantPayload {
	user_id: string;
	username: string;
	session_id: string;
}

// WebSocket connection states
export enum WebSocketState {
	CONNECTING = "connecting",
	CONNECTED = "connected",
	DISCONNECTING = "disconnecting",
	DISCONNECTED = "disconnected",
	ERROR = "error",
}

// Event handler types
export type MessageHandler = (payload: MessagePayload) => void;
export type TypingHandler = (payload: TypingPayload) => void;
export type ParticipantHandler = (payload: ParticipantPayload) => void;
export type ErrorHandler = (error: string) => void;
export type StateChangeHandler = (state: WebSocketState) => void;
