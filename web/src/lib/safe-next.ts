// safeNext returns a same-site path to continue to after login, or null.
// Only plain absolute paths are accepted ("/x", not "//evil.com" or
// "https://evil.com"), so ?next= can't be used as an open redirect.
export function safeNext(raw: string | null): string | null {
	if (!raw || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return null;
	return raw;
}
