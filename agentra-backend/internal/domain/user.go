package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleEditor UserRole = "editor"
	RoleViewer UserRole = "viewer"
)

type User struct {
	ID             uuid.UUID `db:"id"             json:"id"`
	OrganizationID uuid.UUID `db:"organization_id" json:"organization_id"`
	Email          string    `db:"email"          json:"email"`
	PasswordHash   string    `db:"password_hash"  json:"-"`
	Role           UserRole  `db:"role"           json:"role"`
	IsActive       bool      `db:"is_active"      json:"is_active"`
	CreatedAt      time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"     json:"updated_at"`
}
