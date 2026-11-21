// Package orm is a small layer over bun (github.com/uptrace/bun) for
// PostgreSQL, through the pgx driver.
//
// Models are plain structs with bun tags:
//
//	type Invoice struct {
//		bun.BaseModel `bun:"table:invoices"`
//
//		ID        int64      `bun:"id,pk,autoincrement" json:"id"`
//		Number    string     `bun:"number" json:"number"`
//		Items     []*Item    `bun:"rel:has-many,join:id=invoice_id" json:"items,omitempty"`
//		CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
//		UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
//		DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
//	}
//
// and Repository[T] gives every model the everyday operations — Find, Get,
// First, Count, Exists, Paginate, Create, Update, Delete (soft when the model
// has a soft_delete column), ForceDelete and Restore — with ErrNotFound
// instead of sql.ErrNoRows. Queries are narrowed with Scopes (Where,
// OrderBy, Limit, With for relations, ...); anything else is plain bun:
// Repository.Select returns a *bun.SelectQuery and Repository.DB the
// connection, so no query is out of reach.
//
// Transactions are explicit: InTx hands fn a transaction, and a repository
// built on it (WithTx) runs its queries there. Nothing travels in a context.
//
// The package imports only bun, pgx and the standard library, and keeps no
// package-level state: it can be moved to a module of its own unchanged.
package orm
