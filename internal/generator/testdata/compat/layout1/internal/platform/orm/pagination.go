package orm

import (
	"context"

	"github.com/uptrace/bun"
)

// DefaultPerPage is the page size Paginate uses when given none.
const DefaultPerPage = 20

// Page is one page of rows and where it sits among all of them.
type Page[T any] struct {
	Items   []*T  `json:"items"`
	Total   int64 `json:"total"`    // rows matched by every page together
	Page    int   `json:"page"`     // from 1
	PerPage int   `json:"per_page"` // rows per page
}

// Pages is the number of pages; 0 when nothing matched.
func (p Page[T]) Pages() int {
	if p.PerPage <= 0 {
		return 0
	}
	return int((p.Total + int64(p.PerPage) - 1) / int64(p.PerPage))
}

// HasNext reports whether a page follows this one.
func (p Page[T]) HasNext() bool {
	return p.Page < p.Pages()
}

// Paginate returns page (from 1) of the rows the scopes match, perPage at a
// time, with the total count. page < 1 is the first page and perPage < 1 is
// DefaultPerPage; cap perPage yourself when it comes from a client.
func (r *Repository[T]) Paginate(ctx context.Context, page, perPage int, scopes ...Scope) (Page[T], error) {
	page, perPage = pageBounds(page, perPage)
	items := []*T{}
	total, err := r.Select(&items, scopes...).Apply(ForPage(page, perPage)).ScanAndCount(ctx)
	if err != nil {
		return Page[T]{}, r.fail("paginate", err)
	}
	return Page[T]{Items: items, Total: int64(total), Page: page, PerPage: perPage}, nil
}

// ForPage limits a select to one page, for queries that build their own
// select: r.Select(&rows).Apply(orm.ForPage(2, 50)).Scan(ctx).
func ForPage(page, perPage int) Scope {
	page, perPage = pageBounds(page, perPage)
	return func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Limit(int64(perPage)).Offset(int64((page - 1) * perPage))
	}
}

func pageBounds(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	return page, perPage
}
