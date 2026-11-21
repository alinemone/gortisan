package user

import (
	"context"

	"example.com/compat/internal/platform/orm"
)

// Postgres stores user rows in the users table. It implements
// Repository on orm.Repository (bun underneath) and returns this package's
// errors, so the service never depends on the orm.
type Postgres struct {
	rows *orm.Repository[User]
}

var _ Repository = (*Postgres)(nil)

// NewPostgres runs its queries on db: the *bun.DB, or a transaction.
func NewPostgres(db orm.IDB) *Postgres {
	return &Postgres{rows: orm.NewRepository[User](db)}
}

// WithTx returns a copy that runs its queries in tx (see orm.InTx).
func (r *Postgres) WithTx(tx orm.IDB) *Postgres {
	return &Postgres{rows: r.rows.WithTx(tx)}
}

// Find returns ErrNotFound when there is no such row.
func (r *Postgres) Find(ctx context.Context, id int64) (*User, error) {
	m, err := r.rows.Find(ctx, id)
	return m, orm.MapNotFound(err, ErrNotFound)
}

// List returns a page of rows, newest first.
func (r *Postgres) List(ctx context.Context, limit, offset int) ([]*User, error) {
	return r.rows.Get(ctx, orm.OrderBy("?TableAlias.id DESC"), orm.Limit(limit), orm.Offset(offset))
}

// Create inserts m and fills its id and timestamps.
func (r *Postgres) Create(ctx context.Context, m *User) error {
	return r.rows.Create(ctx, m)
}

// Update writes m (only columns, when given) and reads the stored row back
// into it, or returns ErrNotFound.
func (r *Postgres) Update(ctx context.Context, m *User, columns ...string) error {
	return orm.MapNotFound(r.rows.Update(ctx, m, columns...), ErrNotFound)
}

// Delete sets deleted_at; the row is then skipped by every query above.
func (r *Postgres) Delete(ctx context.Context, id int64) error {
	return orm.MapNotFound(r.rows.Delete(ctx, id), ErrNotFound)
}

// More queries are scopes on r.rows, or plain bun on r.rows.DB(), e.g.
//
//	func (r *Postgres) CountSince(ctx context.Context, since time.Time) (int64, error) {
//		return r.rows.Count(ctx, orm.Where("?TableAlias.created_at >= ?", since))
//	}
//
// and relations load with orm.With, once the entity has the relation field:
//
//	m, err := r.rows.Find(ctx, id, orm.With("Items"), orm.With("Items.Statuses"))
