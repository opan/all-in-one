// Mirrors the server's rule (internal/auth.ValidatePassword); the server is
// what actually enforces it.
export const MIN_PASSWORD_LENGTH = 5;

// passwordTooShort counts characters (not UTF-16 units), like the server.
export function passwordTooShort(password: string): boolean {
	return [...password].length < MIN_PASSWORD_LENGTH;
}
