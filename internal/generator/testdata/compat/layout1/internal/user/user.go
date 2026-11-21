// Package user is the user feature: its entity and errors here, the
// use cases in service.go, storage in postgres.go, and HTTP in handler.go,
// dto.go and routes.go. It never imports another feature; internal/app builds
// it and connects it to the rest.
package user

import (
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// User is a row of users. Every field with a bun tag is a column;
// to add one, add a field — e.g. Name string `bun:"name" json:"name"` — and
// the column to the users table.
//
// Relations are fields too, loaded with orm.With in postgres.go, e.g.
//
//	Items []*Item `bun:"rel:has-many,join:id=user_id" json:"items,omitempty"`
//
// bun tag options: pk and autoincrement for the primary key; nullzero writes
// a zero value as NULL (so the database default applies); soft_delete makes
// Delete set the column instead of removing the row.
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        int64      `bun:"id,pk,autoincrement" json:"id"`
	Mobile    string     `bun:"mobile" json:"mobile"`
	Name      string     `bun:"name" json:"name"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// Errors of the user feature, in business terms. handler.go decides
// which HTTP status each one becomes; the service never knows about HTTP.
// Add more as the rules grow, e.g.
//
//	ErrMobileTaken = errors.New("mobile is already taken")
var (
	ErrNotFound = errors.New("user not found")
)
