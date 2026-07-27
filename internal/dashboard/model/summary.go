package model

// Summary is the per-user home dashboard payload. Each section is a pointer so
// it can be omitted (omitempty) for features the user cannot access — the
// frontend renders a stat tile only for the sections that are present.
type Summary struct {
	Listing   *ListingStats   `json:"listing,omitempty"`
	Chat      *ChatStats      `json:"chat,omitempty"`
	Shortener *ShortenerStats `json:"shortener,omitempty"`
}

type ListingStats struct {
	Topics int `json:"topics"`
}

type ChatStats struct {
	Conversations  int `json:"conversations"`
	PendingInvites int `json:"pending_invites"`
}

type ShortenerStats struct {
	Links int `json:"links"`
}
