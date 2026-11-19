package orm

import "github.com/uptrace/bun"

// With loads a relation: a field of the model with a bun rel tag. scopes
// narrow the related rows.
//
//	type Invoice struct {
//		bun.BaseModel `bun:"table:invoices"`
//		ID    int64   `bun:"id,pk,autoincrement"`
//		Items []*Item `bun:"rel:has-many,join:id=invoice_id"`
//	}
//
//	type Item struct {
//		bun.BaseModel `bun:"table:invoice_items"`
//		ID        int64     `bun:"id,pk,autoincrement"`
//		InvoiceID int64     `bun:"invoice_id"`
//		Invoice   *Invoice  `bun:"rel:belongs-to,join:invoice_id=id"`
//		Statuses  []*Status `bun:"rel:has-many,join:id=item_id"`
//	}
//
//	invoice, err := invoices.Find(ctx, id,
//		orm.With("Items", orm.OrderBy("?TableAlias.id")),
//		orm.With("Items.Statuses"), // nested: each item's statuses
//	)
//
// The kinds are has-one, belongs-to, has-many and m2m (see bun's docs).
// has-one and belongs-to are joined into the same query; has-many and m2m
// run one more query per relation, never one per row.
func With(name string, scopes ...Scope) Scope {
	return func(q *bun.SelectQuery) *bun.SelectQuery { return q.Relation(name, scopes...) }
}
