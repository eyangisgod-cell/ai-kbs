package domain

import "time"

// Session is the per-user chat/agent session. Maps to the `sessions`
// table.
type Session struct {
	ID          string
	UserID      string
	Title       string
	KBIDs       []string
	Metadata    []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}