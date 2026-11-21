# gortisan

**English** | [فارسی](README.fa.md)

[![test](https://github.com/alinemone/gortisan/actions/workflows/test.yml/badge.svg)](https://github.com/alinemone/gortisan/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alinemone/gortisan.svg)](https://pkg.go.dev/github.com/alinemone/gortisan)

A code generator for Go HTTP services on [echo v5](https://echo.labstack.com), in the
spirit of Laravel's `artisan` — without a framework behind it.

One project layout, chosen so a beginner never has to decide where a file goes,
and strong enough that a team with a hundred tables and three API versions never
outgrows it. gortisan writes the files and stops there: the project **does not
import gortisan**, gortisan is **not in its `go.mod`**, and nothing runs at request
time that you cannot read in your own repository.

> **Status:** experimental (`v0.0.x`). Commands and templates may still change.

- [Install](#install)
- [Quick start](#quick-start)
- [Project layout](#project-layout)
- [Coming from Laravel](#coming-from-laravel)
- [The `./gortisan` script](#the-gortisan-script)
- [Commands](#commands)
- [API versions](#api-versions)
- [The database layer](#the-database-layer)
- [Validation and errors](#validation-and-errors)
- [Tables and migrations](#tables-and-migrations)
- [Adding auth, Sentry…](#adding-auth-sentry)
- [No magic: the rules](#no-magic-the-rules)
- [Not supported yet](#not-supported-yet)
- [Development](#development)

## Install

Requires Go 1.26 or newer.

```bash
go install github.com/alinemone/gortisan/cmd/gortisan@latest
gortisan version
```

## Quick start

```bash
gortisan create shop github.com/you/shop   # or: gortisan create github.com/you/shop
cd shop
cp .env.example .env          # point it at PostgreSQL and Redis
./gortisan run                # build, run, rebuild and restart on every change
```

```bash
curl localhost:8000/ping      # {"ping":"pong"}
curl localhost:8000/health    # {"status":"pass","checks":{"postgres":"up","redis":"up"}}
```

Add users with a list endpoint:

```bash
./gortisan make:model user --fields "mobile:string,name:string" --soft-delete
./gortisan make:service user
./gortisan make:handler v1/user/list
```

Each command prints what is left to wire by hand — a field in `v1/routes.go`, a
line in `main.go` — and gortisan never rewrites code you wrote.

## Project layout

```
shop/
├── cmd/
│   ├── api/main.go               the HTTP service: builds every dependency and starts it
├── internal/
│   ├── domain/                   THE BUSINESS — no HTTP, no Kafka
│   │   ├── user/
│   │   │   ├── user.go           type User: the table, a struct with bun tags
│   │   │   ├── errors.go         ErrNotFound, ErrMobileTaken…: failures in business terms
│   │   │   ├── store.go          Store: what the service needs from storage
│   │   │   ├── repository.go     Repository: implements Store on orm.Repository[User]
│   │   │   ├── service.go        Service: the rules — the one entry point for everyone else
│   │   │   └── status.go …       enums, caches                          (make:enum user/status …)
│   │   └── sales/                subjects can be grouped once there are many
│   │       ├── order/
│   │       └── payment/
│   ├── api/                      HTTP — an entry point into the domain
│   │   ├── router.go             /ping, /health, and one line per API version
│   │   ├── middleware/
│   │   ├── validator/            reusable checks, by group: address/mobile.go …  (make:validator)
│   │   ├── v1/
│   │   │   ├── routes.go         every v1 endpoint, in one file
│   │   │   ├── users/          handler.go  request.go  response.go
│   │   │   └── sales/orders/
│   │   └── v2/                   only what changed in v2
│   ├── platform/                 INFRASTRUCTURE — no business rules
│   │   ├── config/  server/  validate/  health/   server: http.Server with read/write/idle timeouts
│   │   ├── problem/  trans/      RFC 9457 error bodies; trans.Message reads trans/lang/fa.json
│   │   ├── database/  database/orm/   bun over pgx (postgres)
│   │   ├── trans/lang/fa.json    every text the API shows, by key ("general:entity-not-found")
│   │   └── payment/              clients for other services                  (make:client)
├── tests/arch_test.go            fails `go test` when a layer imports the wrong way
├── gortisan, gortisan.bat, gortisan.ps1
└── go.mod                        echo, bun + pgx, go-redis — only what you picked
```

**Naming.** The domain package is singular — `user`, holding `user.User` and
`user.Service`. The HTTP package is named like its URL — `/v1/users` lives in
`internal/api/v1/users`, holding `users.Handler` and `users.ListRequest`. The two
never clash, so no import needs an alias.

**Why this shape.** It follows the Go team's
[server layout](https://go.dev/doc/modules/layout#server-project) (commands in
`cmd/`, the rest in `internal/`) and the
[Google style guide](https://google.github.io/styleguide/go/best-practices#package-size)
on packages: code used together lives together (all of `user` in one package,
split into focused files), and names do not repeat the package (`user.Service`).
The layers on top — the same split [Ardan Labs' service](https://github.com/ardanlabs/service)
uses (`app/`, `business/`, `foundation/`) — keep that working as the project grows:

- **Twenty subjects or two hundred**, they all live in `domain/` and never mix with
  `config` or `database`; group them (`sales/order`) when one folder gets long.
- **The domain does not know who calls it.** HTTP (`api/`), a queue consumer you add (`worker/`), a
  CLI or gRPC later — each is an entry point calling the same `user.Service`.
- **Dependencies point one way,** and a test enforces it:

  ```
  api, worker  ──►  domain  ──►  platform
  ```

  `tests/arch_test.go` fails when the domain imports `api`, when `platform`
  imports the domain, or when `api/v2` imports `api/v1`.

**Appended files.** Files that collect a kind of declaration — `handler.go`,
`request.go`, `response.go`, `cache.go` — are created
by the first command that needs them and appended to by the next:
`make:handler v1/user/get` adds a method to `handler.go`, `GetRequest` to
`request.go` and `GetResponse` to `response.go`, with the imports they need.
Nothing already in the file changes; a name that already exists is left alone.

## Coming from Laravel

| Laravel | gortisan |
|---|---|
| `app/Models/User.php` | `internal/domain/user/user.go` |
| a service class | `internal/domain/user/service.go` |
| `app/Http/Controllers/Api/V1/UserController.php` | `internal/api/v1/users/handler.go` |
| `app/Http/Requests/…Request.php` | `internal/api/v1/users/request.go` |
| `app/Http/Resources/UserResource.php` | `internal/api/v1/users/response.go` |
| `routes/api.php` | `internal/api/v1/routes.go` |
| `app/Http/Middleware` | `internal/api/middleware/` |
| `config/` | `internal/platform/config/` (and `.env`) |
| `database/migrations`, `php artisan migrate` | none — see [Tables and migrations](#tables-and-migrations) |
| `php artisan serve` | `./gortisan run` |
| `php artisan make:model User -m` | `./gortisan make:model user` |
| `php artisan make:controller` | `./gortisan make:handler v1/user/list` |

## The `./gortisan` script

Every project gets `gortisan`, `gortisan.bat` and `gortisan.ps1` in its root, like
`artisan`:

```bash
./gortisan list                 # macOS / Linux / Git Bash
gortisan.bat list               # cmd
.\gortisan.ps1 list             # PowerShell
```

Goravel's `artisan` runs `go run . artisan`, so the project has to import the
framework. gortisan's script runs `go run github.com/alinemone/gortisan/cmd/gortisan@<version>`,
pinned to the version that created the project: everyone on the team generates
the same code, nobody needs gortisan installed, and `go.mod` stays untouched.

## Commands

| Command | What it does |
|---|---|
| `create <name>` | a new project in `./<name>` |
| `run [command] [-- args]` | builds and runs `cmd/api` (or `cmd/<command>`), restarting on every change to a `.go` file, `go.mod` or `.env`; `--no-watch` runs once |
| `make:model <subject>` | `user.go`, `errors.go`, `store.go`, `repository.go` in `internal/domain/<subject>` *(postgres)* |
| `make:service <subject>` | `service.go`, built on the `Store` when there is one |
| `make:handler <version>/<subject>/<action>` | a method in `internal/api/<version>/<resources>/handler.go`, its input in `request.go`, its output in `response.go`; a new version gets its `routes.go` |
| `make:request <version>/<subject>/<name>` | a type with `Validate` in `request.go` |
| `make:response <version>/<subject>/<name>` | a type in `response.go` |
| `make:validator [<group>/]<name>` | `internal/api/validator/<group>/<name>.go`: a reusable check with `Passes` and `Validate`, and its message key in `internal/platform/trans/lang/fa.json` |
| `make:enum <subject>/<name>` | `<name>.go` in the domain: constants, `String`, `Valid`, `Parse…`, the list of values |
| `make:cache <subject>/<name>` | a typed Redis cache around a loader in the domain's `cache.go` *(redis)* |
| `make:middleware <name>` | `internal/api/middleware/<name>.go` |
| `make:client <name>` | `internal/platform/<name>/client.go`, a JSON HTTP client for another service |
| `make:test <subject>` | `service_test.go`: a service test with an in-memory `Store`, no database |
| `make:test <version>/<subject>/<action>` | `<action>_test.go`: an HTTP test of that handler |
| `add <postgres\|redis>` | adds a piece the project was created without: its files, config, `.env.example` lines and module; prints the two lines to wire |
| `route:list` | every route: method, path, handler and file:line, read from `router.go` and each version's `routes.go` |
| `list`, `help`, `version` | |

A subject may be grouped — `make:model sales/order`, `make:handler v1/sales/order/list`.
`make handler` (with a space) is the same as `make:handler`. The hint printed for a
handler guesses the REST route from the action: `list` → `GET /users`, `get` →
`GET /users/:id`, `create` → `POST /users`, `update` → `PUT /users/:id`, `delete` →
`DELETE /users/:id`.

**Flags**

| Flag | Command | Meaning |
|---|---|---|
| `--module <path>` | `create` | Go module path, the same as the second argument (default: the project name) |
| `--with <list>` | `create` | `postgres`, `redis`, `all` (default) or `none` |
| `--no-tidy` | `create` | skip `go mod tidy` |
| `--no-watch` | `run` | run once |
| `--fields "<list>"` | `make:model` | optional: columns to write in, e.g. `mobile:string,name:string` |
| `--table <name>` | `make:model` | table name (default: the plural, `category` → `categories`) |
| `--soft-delete` | `make:model` | add `deleted_at`; deletes set it and queries skip those rows |
| `--members "<list>"` | `make:enum` | `pending,paid=5,sent`: integers count up from 1 or the previous value; strings default to the name |
| `--type <type>` | `make:enum` | `string` or an integer type (default `int64`) |
| `--type <type>` | `make:cache` | the cached Go type, e.g. `int64`, `[]string`, `*User` (default `string`) |
| `--force` | `make:*` | overwrite whole files that exist |

**Field types** for `--fields`: `string`/`text`, `int`/`int64`/`bigint`,
`int32`/`integer`, `int16`, `float`/`float64`, `bool`, `time`, `json`/`jsonb`; a
trailing `?` makes the column nullable.

## API versions

A version is a package: its own handlers, requests and responses, sharing the
domain with every other version. v1 keeps working while v2 changes.

```go
// internal/api/v1/routes.go — every v1 endpoint in one place
type Services struct {
	User  *user.Service
	Order *order.Service
}

func Register(g *echo.Group, s Services) {
	userHandler := users.NewHandler(s.User)
	g.GET("/users", userHandler.List)
	g.GET("/users/:id", userHandler.Get)

	orderHandler := orders.NewHandler(s.Order)
	g.GET("/orders", orderHandler.List)
}
```

```go
// cmd/api/main.go — wiring in plain sight
users := user.NewService(user.NewRepository(db))
orders := order.NewService(order.NewRepository(db))

api.Register(e, api.Routes{
	Health: health.NewHandler(db, rdb),
	V1:     v1.Services{User: users, Order: orders},
	V2:     v2.Services{User: users},
})
```

`./gortisan make:handler v2/user/list` creates `internal/api/v2/routes.go` the
first time and prints the line that mounts it in `internal/api/router.go`. v2
holds only what changed; the architecture test stops it from importing v1.

## The database layer

PostgreSQL is reached through [bun](https://bun.uptrace.dev) over the
[pgx](https://github.com/jackc/pgx) driver. A model is a plain struct with bun
tags — the struct is the only place a column is declared:

```go
// internal/domain/user/user.go
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        int64      `bun:"id,pk,autoincrement" json:"id"`
	Mobile    string     `bun:"mobile" json:"mobile"`
	Name      string     `bun:"name" json:"name"`
	Note      string     `bun:"-" json:"note"`                 // not a column
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}
```

| bun tag | Means |
|---|---|
| `bun:"table:users"` on `bun.BaseModel` | the table |
| `bun:"name"` | a column |
| `bun:"id,pk,autoincrement"` | the primary key, filled by the database |
| `nullzero` | a zero value is written as `NULL`, so the column default applies |
| `soft_delete` | `Delete` sets the column instead of removing the row; selects skip such rows |
| `bun:"-"` | not a column |
| `rel:has-many,join:id=user_id` (also `has-one`, `belongs-to`, `m2m`) | a relation, loaded on request |

`make:model user --fields "mobile:string,name:string" --soft-delete` writes the
struct and the repository. The table itself is yours to create (see
[Tables and migrations](#tables-and-migrations)); to add a column, add a field
and the column.

`internal/platform/database/orm` is a small layer over bun that lives in your
project. It imports only bun, pgx and the standard library — never your config
or domain — and keeps no globals, so it can be moved to a module of its own
unchanged:

| File | What it holds |
|---|---|
| `orm.go` | package documentation |
| `db.go` | `Options` (plain values), `Open` (pgx + bun, pinged), `NewDB`, `IDB` |
| `repository.go` | `Repository[T]`: `Find`, `First`, `Get`, `Count`, `Exists`, `Create`, `CreateMany`, `Update`, `Delete`, `ForceDelete`, `Restore`, `Select`, `DB`, `WithTx`; fills `created_at` / `updated_at` |
| `query.go` | scopes: `Where`, `WhereOr`, `WhereIn`, `OrderBy`, `Limit`, `Offset`, `Columns`, `WithTrashed`, `OnlyTrashed`, `ForUpdate` |
| `relation.go` | `With(name, scopes...)`: load a relation, nested ones with a dot |
| `pagination.go` | `Paginate` and `Page[T]` (`Total`, `Pages`, `HasNext`), `ForPage` |
| `tx.go` | `InTx`, `InTxOptions`: explicit transactions (savepoints when nested) |
| `errors.go` | `ErrNotFound` (also for `sql.ErrNoRows`), `IsNotFound`, `MapNotFound` |

Its tests build SQL without a database; set `ORM_TEST_DSN` and they also run
every operation against PostgreSQL in a throwaway schema.

The `Service` depends on `Store`, an interface in `store.go` that `Repository`
implements — so a service test can pass a fake store and run without a database.
The repository returns the domain's `ErrNotFound`, never the orm's, so nothing
above it depends on the storage:

```go
// internal/domain/user/repository.go
type Repository struct {
	rows *orm.Repository[User]
}

func NewRepository(db orm.IDB) *Repository {
	return &Repository{rows: orm.NewRepository[User](db)}
}

func (r *Repository) Find(ctx context.Context, id int64) (*User, error) {
	m, err := r.rows.Find(ctx, id)
	return m, orm.MapNotFound(err, ErrNotFound)
}

func (r *Repository) Search(ctx context.Context, name string, page int) (orm.Page[User], error) {
	return r.rows.Paginate(ctx, page, 20,
		orm.Where("?TableAlias.name ILIKE ?", "%"+name+"%"),
		orm.OrderBy("?TableAlias.id DESC"))
}
```

Anything the repository has no method for is plain bun on the same connection:
`r.rows.Select(&dest).ColumnExpr(...).Group(...).Scan(ctx)`, or
`r.rows.DB().NewRaw("SELECT ...", args...).Scan(ctx, &dest)`.

### Relations: invoice items with their statuses

```go
// internal/domain/invoice/invoice.go
type Invoice struct {
	bun.BaseModel `bun:"table:invoices"`
	ID     int64        `bun:"id,pk,autoincrement" json:"id"`
	Number string       `bun:"number" json:"number"`
	Items  []*item.Item `bun:"rel:has-many,join:id=invoice_id" json:"items,omitempty"`
}

// internal/domain/invoice/item/item.go
type Item struct {
	bun.BaseModel `bun:"table:invoice_items"`
	ID        int64            `bun:"id,pk,autoincrement" json:"id"`
	InvoiceID int64            `bun:"invoice_id" json:"invoice_id"`
	Title     string           `bun:"title" json:"title"`
	Statuses  []*status.Status `bun:"rel:has-many,join:id=item_id" json:"statuses,omitempty"`
}

// internal/domain/invoice/item/status/status.go
type Status struct {
	bun.BaseModel `bun:"table:item_statuses"`
	ID     int64  `bun:"id,pk,autoincrement" json:"id"`
	ItemID int64  `bun:"item_id" json:"item_id"`
	Name   string `bun:"name" json:"name"`
}

// internal/domain/invoice/repository.go: the invoice, its items, each with its statuses
func (r *Repository) FindWithItems(ctx context.Context, id int64) (*Invoice, error) {
	m, err := r.rows.Find(ctx, id,
		orm.With("Items", orm.OrderBy("?TableAlias.id")),
		orm.With("Items.Statuses", orm.OrderBy("?TableAlias.id")),
	)
	return m, orm.MapNotFound(err, ErrNotFound)
}
```

This runs three queries — the invoice, its items, their statuses — never one per
row. `make:model invoice`, `make:model invoice/item --table invoice_items` and
`make:model invoice/item/status --table item_statuses` write the three packages;
the relation fields are added by hand.

### Transactions

A transaction is a value, passed explicitly; repositories built on it with
`WithTx` run their queries inside it:

```go
err := orm.InTx(ctx, db, func(ctx context.Context, tx orm.IDB) error {
	if err := invoices.WithTx(tx).Create(ctx, inv); err != nil {
		return err // rolls back
	}
	return payments.WithTx(tx).Create(ctx, p) // nil commits
})
```

## Tables and migrations

gortisan does not create tables and generates no migration runner or seeders.
`make:model` writes the Go side — the struct with its bun tags and the
repository — and the schema is yours: create the tables by hand, or with the
migration tool you prefer ([goose](https://github.com/pressly/goose),
[golang-migrate](https://github.com/golang-migrate/migrate),
[Atlas](https://atlasgo.io), …). A tool that needs its own command can live in
`cmd/migrate`; `./gortisan run` does not start `cmd/migrate`, `cmd/seed` or
`cmd/worker` unless asked.

## Validation and errors

A validator is a small type in `internal/api/validator/<group>/`, shared by
every API version, like salam-gate's validators. `make:validator address/mobile`
writes it; you write `Passes`:

```go
// internal/api/validator/address/mobile.go
type Mobile struct {
	validate.Base // Location, Code (the message key), Status (0 = 422)
	value string
}

func NewMobile(location, value string) *Mobile {
	return &Mobile{Base: validate.Base{Location: location, Code: "address:mobile"}, value: value}
}

func (v *Mobile) Passes(ctx context.Context) (bool, error) {
	return mobileRE.MatchString(v.value), nil // an error means the check could not run
}

func (v *Mobile) Validate(ctx context.Context) error { return validate.Run(ctx, v) }
```

Use it in one of two ways:

```go
// Stop at the first failure, with the validator's status:
if err := address.NewMobile("body.mobile", r.Mobile).Validate(ctx); err != nil {
	return err
}

// Or report every invalid field together, as one 422:
var errs validate.Errors
if err := errs.Check(ctx, address.NewMobile("body.mobile", r.Mobile)); err != nil {
	return err
}
if r.Age < 18 {
	errs.Add("body.age", "general:min", "سن", 18)
}
return errs.Err()
```

A location is `body.<field>` (or just `<field>`), `body.items.0.quantity`,
`query.<name>`, `path.<name>` or `header.<name>`. A validator that should fail
with another status sets it: `v.Status = http.StatusNotFound`.

### Error bodies: RFC 9457

Every error — validation, `echo.NewHTTPError`, a missing route, a 500 — is
[problem details](https://www.rfc-editor.org/rfc/rfc9457) with
`Content-Type: application/problem+json`:

```json
422 {"type": "about:blank", "title": "Unprocessable Entity", "status": 422,
     "detail": "داده‌های ارسال‌شده معتبر نیستند.",
     "errors": [{"pointer": "#/mobile", "code": "address:mobile", "detail": "شماره موبایل معتبر نیست."},
                {"pointer": "#/items/1/quantity", "code": "general:field-required", "detail": "تعداد اجباری است."}]}
404 {"type": "about:blank", "title": "Not Found", "status": 404, "detail": "کاربر مورد نظر یافت نشد."}
500 {"type": "about:blank", "title": "Internal Server Error", "status": 500}
```

`pointer` is a JSON Pointer into the body; path, query and header values use
`parameter`. `code` is the message key, stable for clients to match on.

### Translations

Every text the API shows is in `internal/platform/trans/lang/fa.json`, by key, with `%s` for arguments,
embedded in the binary:

```json
{
  "general:entity-not-found": "%s مورد نظر یافت نشد.",
  "address:mobile": "شماره موبایل معتبر نیست."
}
```

```go
trans.Message("general:entity-not-found", "سفارش") // "سفارش مورد نظر یافت نشد."
```

A missing key is returned as it is, and `internal/platform/trans`'s test fails
when the code uses one (`trans.Message("…")` or `validate.Base{Code: "…"}`) that
`fa.json` does not have, with the file and line.

The domain returns errors in business terms (`errors.go`); the handler decides
the status:

```go
u, err := h.service.Find(ctx, req.ID)
if errors.Is(err, user.ErrNotFound) {
	return echo.NewHTTPError(http.StatusNotFound, trans.Message("general:entity-not-found", "کاربر"))
}
if err != nil {
	return err // a 500; the details go to the log, not the client
}
```

Every request writes one JSON log line with method, path, status and duration;
5xx lines are logged with their cause, and a panic with its stack.

The server is a plain `http.Server` with `ReadHeaderTimeout`, `ReadTimeout`,
`WriteTimeout`, `IdleTimeout` and `MaxHeaderBytes` set (`HTTP_*` in `.env`), so a
slow or idle client cannot hold connections open. Panic recovery is a dozen lines
in `internal/platform/server`, not a library.

## Adding auth, Sentry…

gortisan does not generate them, on purpose: every company does them differently,
and a generated version would have to pick a library for you. They are a few lines
each, added by hand, and they stay explicit:

```bash
./gortisan make:middleware authenticate
```

```go
// internal/api/middleware/authenticate.go
func Authenticate(tokens *auth.Client) echo.MiddlewareFunc { // your own auth SDK
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			who, err := tokens.Verify(c.Request().Context(), c.Request().Header.Get("Authorization"))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			c.Set("caller", who) // or pass it on explicitly
			return next(c)
		}
	}
}

// internal/api/v1/routes.go
g.Use(middleware.Authenticate(s.Tokens))
```

Error reporting (Sentry and the like) goes the same way: a middleware passed to
`server.New(logger, sentryMiddleware)`, or a call in `server`'s error handler.

## No magic: the rules

1. **No import of the generator.** Only the project's own packages and the
   libraries you picked. gortisan's tests check this on every build.
2. **Dependencies are passed, not looked up.** Constructors take what they need and
   `main.go` wires them; nothing is stored in or resolved from `context.Context`.
3. **Layers point one way:** `api`, `worker` → `domain` → `platform`. `tests/arch_test.go`
   enforces it in every project.
4. **Readable code over hidden machinery.** Config reads each variable by name.
   The database goes through bun, a well-known library, and the thin `orm`
   package on top of it is in your repository to read and change.
5. **Transactions are values** — the `tx` of `orm.InTx`, passed to `repo.WithTx(tx)` — and **errors
   carry meaning**: the domain names them, the API maps them to statuses.

## Dependencies

A generated project depends on **echo v5** and, when you pick them, **bun over
pgx** (PostgreSQL) and **go-redis**. Nothing else: Kafka, metrics, error reporting
(Sentry…), tracing (OpenTelemetry…), other databases or queues are yours to add
when you need them — a middleware, a client in `platform/`, a line in
`main.go`. A starter kit that pins third-party SDKs would break when they change.

## Not supported yet

- OpenAPI / Swagger docs

## Development

```bash
go test ./...          # also creates a project for every --with choice, runs every
                       # make:* in it, then go mod tidy, go vet and go test (downloads modules)
go test -short ./...   # unit tests only
```

Templates live in `internal/generator/templates` and are embedded in the binary.
CI runs the full suite on Linux and Windows for every push and pull request.

Releases are git tags: `git tag -a v0.0.4 -m v0.0.4 && git push origin v0.0.4`.

