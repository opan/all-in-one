import { apiGet, apiPost, apiDelete } from "./api";

export interface User {
	id: string;
	username: string;
	email: string;
	name: string;
}

export interface SessionParticipant {
	session_id: string;
	user_id: string;
	joined_at: string;
	left_at: string | null;
	username?: string;
}

export interface ChatSession {
	id: string;
	participant_hash: string;
	status: string;
	created_at: string;
	updated_at: string;
	created_by: string;
	participants?: SessionParticipant[];
	last_message?: ChatMessage;
}

export interface ChatMessage {
	id: string;
	chat_session_id: string;
	user_id: string;
	message: string;
	created_at: string;
	sent_at: string;
	username?: string;
}

export interface ApiResponse<T> {
	success: boolean;
	message: string | null;
	data: T | null;
	error: string | null;
}

export interface CreateSessionRequest {
	participants: string[];
}

export interface SendMessageRequest {
	message: string;
}

/**
 * Get all chat sessions for the current user
 */
export async function getSessions(limit?: number, offset?: number): Promise<ChatSession[]> {
	const params = new URLSearchParams();
	if (limit !== undefined) params.set('limit', limit.toString());
	if (offset !== undefined) params.set('offset', offset.toString());
	
	const url = `/api/v1/chats${params.toString() ? '?' + params.toString() : ''}`;
	const response = await apiGet(url);
	const json = (await response.json()) as ApiResponse<ChatSession[]>;

	if (!json.success || !json.data) {
		throw new Error(json.error || "Failed to fetch sessions");
	}

	return json.data;
}

/**
 * Get a specific chat session by ID
 */
export async function getSession(sessionId: string): Promise<ChatSession> {
	const response = await apiGet(`/api/v1/chats/${sessionId}`);
	const json = (await response.json()) as ApiResponse<ChatSession>;

	if (!json.success || !json.data) {
		throw new Error(json.error || "Failed to fetch session");
	}

	return json.data;
}

/**
 * Create a new chat session
 */
export async function createSession(
	request: CreateSessionRequest,
): Promise<ChatSession> {
	const response = await apiPost("/api/v1/chats", request);
	const json = (await response.json()) as ApiResponse<ChatSession>;

	if (!json.success || !json.data) {
		throw new Error(json.error || "Failed to create session");
	}

	return json.data;
}

/**
 * Leave a chat session
 */
export async function leaveSession(sessionId: string): Promise<void> {
	const response = await apiPost(`/api/v1/chats/${sessionId}/leave`);
	const json = (await response.json()) as ApiResponse<null>;

	if (!json.success) {
		throw new Error(json.error || "Failed to leave session");
	}
}

/**
 * Delete a chat session (admin only)
 */
export async function deleteSession(sessionId: string): Promise<void> {
	const response = await apiDelete(`/api/v1/chats/${sessionId}`);
	const json = (await response.json()) as ApiResponse<null>;

	if (!json.success) {
		throw new Error(json.error || "Failed to delete session");
	}
}

/**
 * Get messages for a specific chat session
 */
export async function getMessages(
	sessionId: string,
	limit = 50,
	offset = 0,
): Promise<ChatMessage[]> {
	const response = await apiGet(
		`/api/v1/chats/${sessionId}/messages?limit=${limit}&offset=${offset}`,
	);
	const json = (await response.json()) as ApiResponse<ChatMessage[]>;

	if (!json.success) {
		throw new Error(json.error || "Failed to fetch messages");
	}

	// Handle null data as empty array (for sessions with no messages)
	return json.data || [];
}

/**
 * Send a message to a chat session
 */
export async function sendMessage(
	sessionId: string,
	request: SendMessageRequest,
): Promise<ChatMessage> {
	const response = await apiPost(
		`/api/v1/chats/${sessionId}/messages`,
		request,
	);
	const json = (await response.json()) as ApiResponse<ChatMessage>;

	if (!json.success || !json.data) {
		throw new Error(json.error || "Failed to send message");
	}

	return json.data;
}

/**
 * Search users by username or email (for creating sessions)
 */
export async function searchUsers(query: string): Promise<User[]> {
	const response = await apiGet(
		`/api/v1/users/search?q=${encodeURIComponent(query)}`,
	);
	const json = (await response.json()) as ApiResponse<User[]>;

	if (!json.success || !json.data) {
		throw new Error(json.error || "Failed to search users");
	}

	return json.data;
}
