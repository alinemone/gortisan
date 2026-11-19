package orm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
)

// TestPostgres runs every operation against a real database when
// ORM_TEST_DSN is set, e.g.
//
//	ORM_TEST_DSN=postgres://postgres:secret@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/platform/database/orm
//
// It works in a schema of its own and drops it afterwards.

type itInvoice struct {
	bun.BaseModel `bun:"table:invoices,alias:i"`

	ID        int64      `bun:"id,pk,autoincrement"`
	Number    string     `bun:"number"`
	Items     []*itItem  `bun:"rel:has-many,join:id=invoice_id"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero"`
}

type itItem struct {
	bun.BaseModel `bun:"table:invoice_items,alias:it"`

	ID        int64       `bun:"id,pk,autoincrement"`
	InvoiceID int64       `bun:"invoice_id"`
	Invoice   *itInvoice  `bun:"rel:belongs-to,join:invoice_id=id"`
	Title     string      `bun:"title"`
	Statuses  []*itStatus `bun:"rel:has-many,join:id=item_id"`
}

type itStatus struct {
	bun.BaseModel `bun:"table:item_statuses,alias:s"`

	ID     int64  `bun:"id,pk,autoincrement"`
	ItemID int64  `bun:"item_id"`
	Name   string `bun:"name"`
}

func TestPostgres(t *testing.T) {
	dsn := os.Getenv("ORM_TEST_DSN")
	if dsn == "" {
		t.Skip("set ORM_TEST_DSN to run against PostgreSQL")
	}
	ctx := context.Background()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	schemaName := fmt.Sprintf("orm_test_%d", time.Now().UnixNano())
	config.RuntimeParams["search_path"] = schemaName
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	db := NewDB(stdlib.OpenDB(*config))
	defer db.Close()

	mustExec(t, db, "CREATE SCHEMA "+schemaName)
	defer func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA "+schemaName+" CASCADE") }()
	mustExec(t, db, `CREATE TABLE invoices (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		number TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		deleted_at TIMESTAMPTZ)`)
	mustExec(t, db, `CREATE TABLE invoice_items (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		invoice_id BIGINT NOT NULL REFERENCES invoices (id),
		title TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE item_statuses (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		item_id BIGINT NOT NULL REFERENCES invoice_items (id),
		name TEXT NOT NULL)`)

	invoices := NewRepository[itInvoice](db)
	items := NewRepository[itItem](db)
	statuses := NewRepository[itStatus](db)

	// Create fills the id and the timestamps.
	inv := &itInvoice{Number: "INV-1"}
	check(t, invoices.Create(ctx, inv))
	if inv.ID == 0 || inv.CreatedAt.IsZero() || inv.UpdatedAt.IsZero() {
		t.Fatalf("Create did not read the row back: %+v", inv)
	}
	lines := []*itItem{
		{InvoiceID: inv.ID, Title: "pen"},
		{InvoiceID: inv.ID, Title: "ink"},
	}
	check(t, items.CreateMany(ctx, lines))
	if lines[0].ID == 0 || lines[1].ID == 0 {
		t.Fatalf("CreateMany did not fill the ids: %+v %+v", lines[0], lines[1])
	}
	check(t, statuses.CreateMany(ctx, []*itStatus{
		{ItemID: lines[0].ID, Name: "ordered"}, {ItemID: lines[0].ID, Name: "shipped"}, {ItemID: lines[1].ID, Name: "ordered"},
	}))

	// Relations: the invoice's items, each with its statuses.
	got, err := invoices.Find(ctx, inv.ID, With("Items", OrderBy("?TableAlias.id")), With("Items.Statuses", OrderBy("?TableAlias.id")))
	check(t, err)
	if len(got.Items) != 2 || got.Items[0].Title != "pen" || len(got.Items[0].Statuses) != 2 || got.Items[0].Statuses[1].Name != "shipped" || len(got.Items[1].Statuses) != 1 {
		t.Fatalf("relations not loaded: %+v", got)
	}
	item, err := items.Find(ctx, lines[1].ID, With("Invoice"))
	check(t, err)
	if item.Invoice == nil || item.Invoice.Number != "INV-1" {
		t.Fatalf("belongs-to not loaded: %+v", item)
	}

	// Queries.
	n, err := items.Count(ctx, Where("?TableAlias.invoice_id = ?", inv.ID))
	check(t, err)
	exists, err := items.Exists(ctx, Where("?TableAlias.title = ?", "ink"))
	check(t, err)
	first, err := items.First(ctx, OrderBy("?TableAlias.id DESC"))
	check(t, err)
	all, err := items.Get(ctx, WhereIn("id", []int64{lines[0].ID}))
	check(t, err)
	if n != 2 || !exists || first.Title != "ink" || len(all) != 1 {
		t.Fatalf("Count = %d, Exists = %v, First = %+v, Get = %d rows", n, exists, first, len(all))
	}
	if _, err := items.Find(ctx, int64(1_000_000)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find of a missing row: %v", err)
	}
	if _, err := items.First(ctx, Where("?TableAlias.title = ?", "none")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("First with no match: %v", err)
	}

	// Update writes the columns and refreshes updated_at.
	before := inv.UpdatedAt
	createdAt := inv.CreatedAt
	inv.Number = "INV-1A"
	time.Sleep(10 * time.Millisecond)
	check(t, invoices.Update(ctx, inv, "number"))
	if !inv.UpdatedAt.After(before) || !inv.CreatedAt.Equal(createdAt) {
		t.Fatalf("Update: updated_at %v -> %v, created_at %v -> %v", before, inv.UpdatedAt, createdAt, inv.CreatedAt)
	}
	inv.Number = "INV-1B"
	check(t, invoices.Update(ctx, inv)) // every column
	if stored, _ := invoices.Find(ctx, inv.ID); stored == nil || stored.Number != "INV-1B" {
		t.Fatalf("Update without columns did not write: %+v", stored)
	}
	if err := invoices.Update(ctx, &itInvoice{ID: 1_000_000, Number: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update of a missing row: %v", err)
	}

	// Soft delete, restore, force delete.
	check(t, invoices.Delete(ctx, inv.ID))
	if _, err := invoices.Find(ctx, inv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a soft-deleted row must not be found: %v", err)
	}
	trashed, err := invoices.Find(ctx, inv.ID, WithTrashed())
	check(t, err)
	if trashed.DeletedAt == nil {
		t.Fatal("deleted_at was not set")
	}
	if c, _ := invoices.Count(ctx, OnlyTrashed()); c != 1 {
		t.Fatalf("OnlyTrashed count = %d", c)
	}
	if err := invoices.Delete(ctx, inv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a deleted row: %v", err)
	}
	if err := invoices.Update(ctx, inv, "number"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("updating a deleted row: %v", err)
	}
	check(t, invoices.Restore(ctx, inv.ID))
	if _, err := invoices.Find(ctx, inv.ID); err != nil {
		t.Fatalf("restored row: %v", err)
	}
	if err := invoices.Restore(ctx, inv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restoring a live row: %v", err)
	}

	// Transactions: an error or a panic rolls back, nil commits.
	failed := errors.New("stop")
	err = InTx(ctx, db, func(ctx context.Context, tx IDB) error {
		check(t, invoices.WithTx(tx).Create(ctx, &itInvoice{Number: "ROLLED-BACK"}))
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("InTx returned %v", err)
	}
	func() {
		defer func() { _ = recover() }()
		_ = InTx(ctx, db, func(ctx context.Context, tx IDB) error {
			check(t, invoices.WithTx(tx).Create(ctx, &itInvoice{Number: "ROLLED-BACK"}))
			panic("boom")
		})
	}()
	check(t, InTx(ctx, db, func(ctx context.Context, tx IDB) error {
		return invoices.WithTx(tx).Create(ctx, &itInvoice{Number: "COMMITTED"})
	}))
	if ok, _ := invoices.Exists(ctx, Where("?TableAlias.number = ?", "ROLLED-BACK")); ok {
		t.Fatal("a rolled back insert is visible")
	}
	if ok, _ := invoices.Exists(ctx, Where("?TableAlias.number = ?", "COMMITTED")); !ok {
		t.Fatal("a committed insert is missing")
	}

	// Pagination.
	for i := range 3 {
		check(t, invoices.Create(ctx, &itInvoice{Number: fmt.Sprintf("P-%d", i)}))
	}
	page, err := invoices.Paginate(ctx, 2, 2, OrderBy("?TableAlias.id"))
	check(t, err)
	if page.Total != 5 || len(page.Items) != 2 || page.Pages() != 3 || !page.HasNext() {
		t.Fatalf("Paginate = total %d, %d items, %d pages", page.Total, len(page.Items), page.Pages())
	}

	// Hard delete: children first.
	_, err = db.NewDelete().Model((*itStatus)(nil)).Where("true").Exec(ctx)
	check(t, err)
	_, err = db.NewDelete().Model((*itItem)(nil)).Where("true").Exec(ctx)
	check(t, err)
	check(t, invoices.ForceDelete(ctx, inv.ID))
	if _, err := invoices.Find(ctx, inv.ID, WithTrashed()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a force-deleted row is still there: %v", err)
	}
}

func mustExec(t *testing.T, db *bun.DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
