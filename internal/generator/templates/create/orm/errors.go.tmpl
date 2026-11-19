package orm

import (
	"database/sql"
	"errors"
)

// Error is an error of this package. It is a string type so its values can
// be constants: the package keeps no variables.
type Error string

func (e Error) Error() string { return string(e) }

// ErrNotFound is returned when a query that needs a row finds none: Find,
// First, Update, Delete, ForceDelete and Restore.
const ErrNotFound Error = "orm: not found"

// IsNotFound reports whether err means "no row": ErrNotFound, or
// sql.ErrNoRows from a plain bun query such as
// db.NewSelect().Model(&m).Where(...).Scan(ctx).
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}

// MapNotFound returns target when err means "no row", and err otherwise. A
// domain repository uses it to return its own error, so nothing above the
// repository depends on this package:
//
//	m, err := r.rows.Find(ctx, id)
//	return m, orm.MapNotFound(err, ErrNotFound)
func MapNotFound(err, target error) error {
	if IsNotFound(err) {
		return target
	}
	return err
}

// noRow turns sql.ErrNoRows into ErrNotFound.
func noRow(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
