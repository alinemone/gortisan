package orm

import "github.com/uptrace/bun"

// Scope changes a select: a condition, an order, a limit, a relation to
// load. It has the type bun's own Relation and Apply take, so a Scope works
// anywhere bun expects one and any bun function of that shape is a Scope.
//
//	orders, err := r.Get(ctx,
//		orm.Where("?TableAlias.user_id = ?", userID),
//		orm.OrderBy("?TableAlias.id DESC"),
//		orm.Limit(20),
//	)
//
// Conditions use ? for arguments; bun quotes the values. ?TableAlias is the
// model's alias: write it before column names when a query joins relations.
// Column names and order expressions are written as given, so never build
// them from user input.
type Scope = func(*bun.SelectQuery) *bun.SelectQuery

// Apply runs scopes on q in order.
func Apply(q *bun.SelectQuery, scopes ...Scope) *bun.SelectQuery {
	for _, scope := range scopes {
		if scope != nil {
			q = scope(q)
		}
	}
	return q
}

// Where adds a condition; conditions are joined with AND.
func Where(query string, args ...any) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.Where(query, args...) }
}

// WhereOr adds a condition joined with OR to the ones before it.
func WhereOr(query string, args ...any) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.WhereOr(query, args...) }
}

// WhereIn keeps the rows whose column is one of values (a slice).
func WhereIn(column string, values any) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.? IN (?)", bun.Ident(column), bun.In(values))
	}
}

// OrderBy adds an ORDER BY expression, e.g. "?TableAlias.created_at DESC".
func OrderBy(expr string, args ...any) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.OrderExpr(expr, args...) }
}

// Limit returns at most n rows; n <= 0 means no limit.
func Limit(n int) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery {
		if n <= 0 {
			return q
		}
		return q.Limit(int64(n))
	}
}

// Offset skips the first n rows.
func Offset(n int) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery {
		if n <= 0 {
			return q
		}
		return q.Offset(int64(n))
	}
}

// Columns selects only these columns of the model.
func Columns(columns ...string) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.Column(columns...) }
}

// WithTrashed includes soft-deleted rows.
func WithTrashed() Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.WhereAllWithDeleted() }
}

// OnlyTrashed keeps only soft-deleted rows.
func OnlyTrashed() Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.WhereDeleted() }
}

// ForUpdate locks the selected rows until the transaction ends.
func ForUpdate() Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.For("UPDATE") }
}
