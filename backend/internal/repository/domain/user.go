package domain

import "time"

// User is the AI-KBS user record. Maps to the `users` table.
type User struct {
	ID            string
	Username      string
	Email         string
	DisplayName   string
	PasswordHash  string
	IsSuperAdmin  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedFlag   bool
}