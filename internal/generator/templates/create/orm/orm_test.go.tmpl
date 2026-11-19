package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
)

// These tests build SQL without a database. integration_test.go runs the
// same operations against PostgreSQL when ORM_TEST_DSN is set.

type testAuthor struct {
	bun.BaseModel `bun:"table:authors,alias:a"`

	ID   int64  `bun:"id,pk,autoincrement"`
	Name string `bun:"name"`
}

type testPost struct {
	bun.BaseModel `bun:"table:posts,alias:p"`

	ID        int64       `bun:"id,pk,autoincrement"`
	AuthorID  int64       `bun:"author_id"`
	Author    *testAuthor `bun:"rel:belongs-to,join:author_id=id"`
	Title     string      `bun:"title"`
	CreatedAt time.Time   `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt *time.Time  `bun:"updated_at,nullzero"`
	DeletedAt *time.Time  `bun:"deleted_at,soft_delete,nullzero"`
}

// offlineDB is a *bun.DB that never connects: building SQL needs none.
func offlineDB(t *testing.T) *bun.DB {
	t.Helper()
	config, err := pgx.ParseConfig("postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	db := NewDB(stdlib.OpenDB(*config))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOptionsDSN(t *testing.T) {
	dsn := Options{Host: "db.local", Database: "shop", User: "app", Password: "p@ss:w/rd", Schema: "sales"}.DSN()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if u.Host != "db.local:5432" || u.Path != "/shop" || password != "p@ss:w/rd" ||
		u.Query().Get("sslmode") != "prefer" || u.Query().Get("search_path") != "sales" {
		t.Fatalf("DSN = %s", dsn)
	}
	if _, err := pgx.ParseConfig(dsn); err != nil {
		t.Fatalf("pgx rejects %s: %v", dsn, err)
	}
}

func TestSelectSkipsSoftDeletedRows(t *testing.T) {
	posts := NewRepository[testPost](offlineDB(t))
	cases := []struct {
		scopes []Scope
		want   []string
		not    []string
	}{
		{nil, []string{`FROM "posts" AS "p"`, `"p"."deleted_at" IS NULL`}, nil},
		{scopes: []Scope{WithTrashed()}, not: []string{"IS NULL"}},
		{[]Scope{OnlyTrashed()}, []string{`"p"."deleted_at" IS NOT NULL`}, nil},
		{
			[]Scope{Where("?TableAlias.title = ?", "it's"), OrderBy("?TableAlias.id DESC"), Limit(5), Offset(10)},
			[]string{`"p".title = 'it''s'`, `ORDER BY "p".id DESC`, "LIMIT 5", "OFFSET 10"},
			nil,
		},
		{[]Scope{WhereIn("id", []int64{1, 2})}, []string{`"p"."id" IN (1, 2)`}, nil},
		{scopes: []Scope{Limit(0), Offset(-1)}, not: []string{"LIMIT", "OFFSET"}},
		{[]Scope{ForPage(3, 10)}, []string{"LIMIT 10", "OFFSET 20"}, nil},
		{[]Scope{With("Author")}, []string{`LEFT JOIN "authors" AS "author"`}, nil},
	}
	for i, c := range cases {
		sql := posts.Select(nil, c.scopes...).String()
		for _, want := range c.want {
			if !strings.Contains(sql, want) {
				t.Errorf("case %d: %s\nlacks %s", i, sql, want)
			}
		}
		for _, not := range c.not {
			if strings.Contains(sql, not) {
				t.Errorf("case %d: %s\nmust not contain %s", i, sql, not)
			}
		}
	}
}

func TestTouch(t *testing.T) {
	posts := NewRepository[testPost](offlineDB(t))
	given := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	created := &testPost{CreatedAt: given}
	posts.touch(created, true)
	if !created.CreatedAt.Equal(given) || created.UpdatedAt == nil {
		t.Fatalf("create must keep a given created_at and fill updated_at: %+v", created)
	}

	updated := &testPost{UpdatedAt: &given}
	posts.touch(updated, false)
	if updated.UpdatedAt.Equal(given) || !updated.CreatedAt.IsZero() {
		t.Fatalf("update must refresh updated_at only: %+v", updated)
	}

	authors := NewRepository[testAuthor](offlineDB(t))
	authors.touch(&testAuthor{}, true) // no timestamp columns: nothing to do
}

func TestNotFound(t *testing.T) {
	errDomain := errors.New("post not found")
	wrapped := fmt.Errorf("lookup: %w", sql.ErrNoRows)
	for _, err := range []error{ErrNotFound, sql.ErrNoRows, wrapped} {
		if !IsNotFound(err) || MapNotFound(err, errDomain) != errDomain {
			t.Errorf("%v must map to the domain error", err)
		}
	}
	other := errors.New("connection refused")
	if IsNotFound(other) || MapNotFound(other, errDomain) != other || MapNotFound(nil, errDomain) != nil {
		t.Error("other errors and nil must pass through")
	}
	posts := NewRepository[testPost](offlineDB(t))
	if err := posts.fail("find", sql.ErrNoRows); err != ErrNotFound {
		t.Errorf("fail(sql.ErrNoRows) = %v", err)
	}
	if err := posts.fail("find", other); !errors.Is(err, other) || !strings.Contains(err.Error(), "posts") {
		t.Errorf("fail(other) = %v", err)
	}
}

func TestPage(t *testing.T) {
	for _, c := range []struct {
		total         int64
		page, perPage int
		pages         int
		hasNext       bool
	}{
		{0, 1, 20, 0, false},
		{20, 1, 20, 1, false},
		{21, 1, 20, 2, true},
		{21, 2, 20, 2, false},
	} {
		p := Page[testPost]{Total: c.total, Page: c.page, PerPage: c.perPage}
		if p.Pages() != c.pages || p.HasNext() != c.hasNext {
			t.Errorf("%+v: Pages = %d, HasNext = %v", c, p.Pages(), p.HasNext())
		}
	}
	if page, perPage := pageBounds(0, 0); page != 1 || perPage != DefaultPerPage {
		t.Errorf("pageBounds(0, 0) = %d, %d", page, perPage)
	}
}

func TestSinglePK(t *testing.T) {
	type noKey struct {
		bun.BaseModel `bun:"table:logs"`
		Line          string `bun:"line"`
	}
	if err := NewRepository[noKey](offlineDB(t)).singlePK(); err == nil {
		t.Error("a model without a primary key cannot be found by id")
	}
	if err := NewRepository[testAuthor](offlineDB(t)).Restore(t.Context(), 1); err == nil || !strings.Contains(err.Error(), "soft_delete") {
		t.Errorf("Restore without a soft_delete column = %v", err)
	}
}
