package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// The timestamp columns Repository fills, when the model has them (as a
// time.Time or *time.Time field).
const (
	CreatedAtColumn = "created_at"
	UpdatedAtColumn = "updated_at"
)

// Repository reads and writes the table of the model T, a struct with bun
// tags (see the package documentation). It is safe for concurrent use.
//
// Lookups by id need T to have exactly one primary key column. A model with
// a soft_delete column (`bun:"deleted_at,soft_delete,nullzero"`) is
// soft-deleted: Delete sets the column and every select skips those rows,
// unless it uses WithTrashed or OnlyTrashed.
type Repository[T any] struct {
	db    IDB
	table *schema.Table
}

// NewRepository returns a repository of T that runs its queries on db: the
// *bun.DB, or a transaction.
func NewRepository[T any](db IDB) *Repository[T] {
	return &Repository[T]{db: db, table: db.Dialect().Tables().Get(reflect.TypeFor[T]())}
}

// WithTx returns a copy that runs its queries on tx.
func (r *Repository[T]) WithTx(tx IDB) *Repository[T] {
	return &Repository[T]{db: tx, table: r.table}
}

// DB is the connection or transaction the repository runs on, for queries
// it has no method for.
func (r *Repository[T]) DB() IDB {
	return r.db
}

// Table is the table name.
func (r *Repository[T]) Table() string {
	return r.table.Name
}

// Select starts a bun select of T's table with scopes applied. dest is
// where Scan writes: a *T, a *[]T or *[]*T, or nil to count or check
// existence only.
//
//	var totals []struct{ Status string; N int }
//	err := r.Select(nil).ColumnExpr("status, count(*) AS n").Group("status").Scan(ctx, &totals)
func (r *Repository[T]) Select(dest any, scopes ...Scope) *bun.SelectQuery {
	if dest == nil {
		dest = (*T)(nil)
	}
	return Apply(r.db.NewSelect().Model(dest), scopes...)
}

// Find returns the row whose primary key is id, or ErrNotFound. scopes may
// load relations, e.g. Find(ctx, id, With("Items")).
func (r *Repository[T]) Find(ctx context.Context, id any, scopes ...Scope) (*T, error) {
	if err := r.singlePK(); err != nil {
		return nil, err
	}
	m := new(T)
	if err := r.Select(m, scopes...).Where("?TablePKs = ?", id).Limit(1).Scan(ctx); err != nil {
		return nil, r.fail("find", err)
	}
	return m, nil
}

// First returns the first row the scopes match, or ErrNotFound.
func (r *Repository[T]) First(ctx context.Context, scopes ...Scope) (*T, error) {
	m := new(T)
	if err := r.Select(m, scopes...).Limit(1).Scan(ctx); err != nil {
		return nil, r.fail("first", err)
	}
	return m, nil
}

// Get returns every row the scopes match; an empty slice when none do.
func (r *Repository[T]) Get(ctx context.Context, scopes ...Scope) ([]*T, error) {
	rows := []*T{}
	if err := r.Select(&rows, scopes...).Scan(ctx); err != nil {
		return nil, r.fail("get", err)
	}
	return rows, nil
}

// Count returns how many rows the scopes match.
func (r *Repository[T]) Count(ctx context.Context, scopes ...Scope) (int64, error) {
	n, err := r.Select(nil, scopes...).Count(ctx)
	if err != nil {
		return 0, r.fail("count", err)
	}
	return int64(n), nil
}

// Exists reports whether the scopes match any row.
func (r *Repository[T]) Exists(ctx context.Context, scopes ...Scope) (bool, error) {
	ok, err := r.Select(nil, scopes...).Exists(ctx)
	if err != nil {
		return false, r.fail("exists", err)
	}
	return ok, nil
}

// Create inserts m and reads the stored row back into it, so the id and
// the database defaults are filled. created_at and updated_at are set to
// now when they are zero.
func (r *Repository[T]) Create(ctx context.Context, m *T) error {
	r.touch(m, true)
	if _, err := r.db.NewInsert().Model(m).Returning("*").Exec(ctx); err != nil {
		return r.fail("create", err)
	}
	return nil
}

// CreateMany inserts models in one statement and reads the stored rows back.
func (r *Repository[T]) CreateMany(ctx context.Context, models []*T) error {
	if len(models) == 0 {
		return nil
	}
	for _, m := range models {
		r.touch(m, true)
	}
	if _, err := r.db.NewInsert().Model(&models).Returning("*").Exec(ctx); err != nil {
		return r.fail("create", err)
	}
	return nil
}

// Update writes m to the row with m's primary key and reads the stored row
// back into m, or returns ErrNotFound (also for a soft-deleted row).
// updated_at is set to now. With columns, only those are written, e.g.
// Update(ctx, m, "status"); without, every column but the primary key,
// created_at and the soft_delete column.
func (r *Repository[T]) Update(ctx context.Context, m *T, columns ...string) error {
	if err := r.singlePK(); err != nil {
		return err
	}
	r.touch(m, false)
	q := r.db.NewUpdate().Model(m).WherePK().Returning("*")
	if len(columns) > 0 {
		if r.hasTime(UpdatedAtColumn) && !slices.Contains(columns, UpdatedAtColumn) {
			columns = append(slices.Clip(columns), UpdatedAtColumn)
		}
		q = q.Column(columns...)
	} else {
		var skip []string
		if r.table.HasField(CreatedAtColumn) {
			skip = append(skip, CreatedAtColumn)
		}
		if f := r.table.SoftDeleteField; f != nil {
			skip = append(skip, f.Name)
		}
		if len(skip) > 0 {
			q = q.ExcludeColumn(skip...)
		}
	}
	res, err := q.Exec(ctx)
	return r.affected("update", res, err)
}

// Delete removes the row whose primary key is id — or, when T has a
// soft_delete column, sets it — or returns ErrNotFound.
func (r *Repository[T]) Delete(ctx context.Context, id any) error {
	if err := r.singlePK(); err != nil {
		return err
	}
	res, err := r.db.NewDelete().Model(new(T)).Where("?TablePKs = ?", id).Exec(ctx)
	return r.affected("delete", res, err)
}

// ForceDelete removes the row whose primary key is id even when T is
// soft-deleted, or returns ErrNotFound.
func (r *Repository[T]) ForceDelete(ctx context.Context, id any) error {
	if err := r.singlePK(); err != nil {
		return err
	}
	res, err := r.db.NewDelete().Model(new(T)).Where("?TablePKs = ?", id).ForceDelete().Exec(ctx)
	return r.affected("delete", res, err)
}

// Restore clears the soft_delete column of the deleted row whose primary
// key is id, or returns ErrNotFound.
func (r *Repository[T]) Restore(ctx context.Context, id any) error {
	if err := r.singlePK(); err != nil {
		return err
	}
	f := r.table.SoftDeleteField
	if f == nil {
		return fmt.Errorf("orm: %s has no soft_delete column to restore", r.table.Name)
	}
	q := r.db.NewUpdate().Model(new(T)).
		Set("? = NULL", bun.Ident(f.Name)).
		Where("?TablePKs = ?", id).
		WhereDeleted()
	if r.hasTime(UpdatedAtColumn) {
		q = q.Set("? = ?", bun.Ident(UpdatedAtColumn), time.Now())
	}
	res, err := q.Exec(ctx)
	return r.affected("restore", res, err)
}

// affected turns "no row changed" into ErrNotFound.
func (r *Repository[T]) affected(op string, res sql.Result, err error) error {
	if err != nil {
		return r.fail(op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return r.fail(op, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// fail returns ErrNotFound for "no row" and err with the operation and
// table otherwise.
func (r *Repository[T]) fail(op string, err error) error {
	if err = noRow(err); errors.Is(err, ErrNotFound) {
		return err
	}
	return fmt.Errorf("orm: %s %s: %w", op, r.table.Name, err)
}

func (r *Repository[T]) singlePK() error {
	if len(r.table.PKs) != 1 {
		return fmt.Errorf("orm: %s needs exactly one primary key column (bun:\",pk\") to be found by id; it has %d", r.table.TypeName, len(r.table.PKs))
	}
	return nil
}

// hasTime reports whether column is a time.Time or *time.Time field.
func (r *Repository[T]) hasTime(column string) bool {
	f, ok := r.table.FieldMap[column]
	return ok && (f.IndirectType == reflect.TypeFor[time.Time]())
}

// touch sets updated_at to now, and on create also created_at, leaving
// values the caller set on create alone.
func (r *Repository[T]) touch(m *T, creating bool) {
	now := time.Now()
	v := reflect.ValueOf(m).Elem()
	if creating {
		r.setTime(v, CreatedAtColumn, now, true)
	}
	r.setTime(v, UpdatedAtColumn, now, creating)
}

func (r *Repository[T]) setTime(v reflect.Value, column string, now time.Time, onlyZero bool) {
	if !r.hasTime(column) {
		return
	}
	fv := r.table.FieldMap[column].Value(v)
	switch fv.Kind() {
	case reflect.Pointer:
		if onlyZero && !fv.IsNil() {
			return
		}
		fv.Set(reflect.ValueOf(&now))
	default:
		if onlyZero && !fv.Interface().(time.Time).IsZero() {
			return
		}
		fv.Set(reflect.ValueOf(now))
	}
}
