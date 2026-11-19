# gortisan

[![test](https://github.com/alinemone/gortisan/actions/workflows/test.yml/badge.svg)](https://github.com/alinemone/gortisan/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alinemone/gortisan.svg)](https://pkg.go.dev/github.com/alinemone/gortisan)

A code generator for Go HTTP services on [echo](https://echo.labstack.com), in the
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
- [Kafka: workers and CDC](#kafka-workers-and-cdc)
- [Adding auth, Sentry, translations…](#adding-auth-sentry-translations)
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
gortisan create shop --module github.com/you/shop
cd shop
cp .env.example .env          # point it at PostgreSQL and Redis
./gortisan run                # build, run, rebuild and restart on every change
```

```bash
curl localhost:8080/ping      # {"ping":"pong"}
curl localhost:8080/health    # {"status":"pass","checks":{"postgres":"up","redis":"up"}}
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
│   └── worker/main.go            the Kafka workers, one per process            (kafka)
├── internal/
│   ├── domain/                   THE BUSINESS — no HTTP, no Kafka
│   │   ├── user/
│   │   │   ├── user.go           type User: the table, a struct with db tags
│   │   │   ├── errors.go         ErrNotFound, ErrMobileTaken…: failures in business terms
│   │   │   ├── store.go          Store: what the service needs from storage
│   │   │   ├── repository.go     Repository: implements Store; the table mapping is generated at its end
│   │   │   ├── service.go        Service: the rules — the one entry point for everyone else
│   │   │   └── status.go …       enums, validators, caches, CDC          (make:enum user/status …)
│   │   └── sales/                subjects can be grouped once there are many
│   │       ├── order/
│   │       └── payment/
│   ├── api/                      HTTP — an entry point into the domain
│   │   ├── router.go             /ping, /health, /metrics, and one line per API version
│   │   ├── middleware/
│   │   ├── v1/
│   │   │   ├── routes.go         every v1 endpoint, in one file
│   │   │   ├── userapi/          handler.go  request.go  response.go
│   │   │   └── sales/orderapi/
│   │   └── v2/                   only what changed in v2
│   ├── worker/                   Kafka — another entry point into the domain   (kafka)
│   │   └── userworker/
│   ├── platform/                 INFRASTRUCTURE — no business rules
│   │   ├── config/  server/  validate/  health/
│   │   ├── database/  database/orm/                                           (postgres)
│   │   ├── cache/  kafka/  cdc/  metrics/                                     (by --with)
│   │   └── payment/              clients for other services                  (make:client)
│   └── arch_test.go              fails `go test` when a layer imports the wrong way
├── migrations/                   plain SQL, *.up.sql / *.down.sql
├── gortisan, gortisan.bat, gortisan.ps1
└── go.mod                        echo, pgx, go-redis, kafka-go, prometheus — only what you picked
```

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
- **The domain does not know who calls it.** HTTP (`api/`), Kafka (`worker/`), a
  CLI or gRPC later — each is an entry point calling the same `user.Service`.
- **Dependencies point one way,** and a test enforces it:

  ```
  api, worker  ──►  domain  ──►  platform
  ```

  `internal/arch_test.go` fails when the domain imports `api`, when `platform`
  imports the domain, or when `api/v2` imports `api/v1`.

**Appended files.** Files that collect a kind of declaration — `handler.go`,
`request.go`, `response.go`, `validator.go`, `cache.go`, `worker.go` — are created
by the first command that needs them and appended to by the next:
`make:handler v1/user/get` adds a method to `handler.go`, `GetRequest` to
`request.go` and `GetResponse` to `response.go`, with the imports they need.
Nothing already in the file changes; a name that already exists is left alone.

## Coming from Laravel

| Laravel | gortisan |
|---|---|
| `app/Models/User.php` | `internal/domain/user/user.go` |
| a service class | `internal/domain/user/service.go` |
| `app/Http/Controllers/Api/V1/UserController.php` | `internal/api/v1/userapi/handler.go` |
| `app/Http/Requests/…Request.php` | `internal/api/v1/userapi/request.go` |
| `app/Http/Resources/UserResource.php` | `internal/api/v1/userapi/response.go` |
| `routes/api.php` | `internal/api/v1/routes.go` |
| `app/Http/Middleware` | `internal/api/middleware/` |
| `app/Jobs`, queue workers | `internal/worker/`, `cmd/worker` |
| `config/` | `internal/platform/config/` (and `.env`) |
| `database/migrations` | `migrations/` |
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
| `run [command] [-- args]` | builds and runs `cmd/api` (or `cmd/<command>`), regenerating table mappings and restarting on every change to a `.go` file, `go.mod` or `.env`; `--no-watch` runs once |
| `generate` | writes the table mapping of every `//gortisan:table` struct into its `repository.go`; `--check` fails if one is out of date (for CI) |
| `make:model <subject>` | `user.go`, `errors.go`, `store.go`, `repository.go` in `internal/domain/<subject>`, and a create-table migration *(postgres)* |
| `make:service <subject>` | `service.go`, built on the `Store` when there is one |
| `make:handler <version>/<subject>/<action>` | a method in `internal/api/<version>/<subject>api/handler.go`, its input in `request.go`, its output in `response.go`; a new version gets its `routes.go` |
| `make:request <version>/<subject>/<name>` | a type with `Validate` in `request.go` |
| `make:response <version>/<subject>/<name>` | a type in `response.go` |
| `make:validator <subject>/<name>` | a reusable rule (`validate.Rule`) in the domain's `validator.go` |
| `make:enum <subject>/<name>` | `<name>.go` in the domain: constants, `String`, `Valid`, `Parse…`, the list of values |
| `make:cache <subject>/<name>` | a typed Redis cache around a loader in the domain's `cache.go` *(redis)* |
| `make:worker <subject>/<name>` | a Kafka consumer in `internal/worker/<subject>worker`, and `cmd/worker` *(kafka)* |
| `make:cdc <subject>` | `cdc.go`: change events of the model to Kafka *(kafka, postgres)* |
| `make:middleware <name>` | `internal/api/middleware/<name>.go` |
| `make:client <name>` | `internal/platform/<name>/client.go`, a JSON HTTP client for another service |
| `make:migration <name>` | empty `migrations/<time>_<name>.{up,down}.sql` |
| `list`, `help`, `version` | |

A subject may be grouped — `make:model sales/order`, `make:handler v1/sales/order/list`.
`make handler` (with a space) is the same as `make:handler`. The hint printed for a
handler guesses the REST route from the action: `list` → `GET /users`, `get` →
`GET /users/:id`, `create` → `POST /users`, `update` → `PUT /users/:id`, `delete` →
`DELETE /users/:id`.

**Flags**

| Flag | Command | Meaning |
|---|---|---|
| `--module <path>` | `create` | Go module path (default: the project name) |
| `--with <list>` | `create` | any of `postgres`, `redis`, `kafka`, `metrics`; `all`; `none` (default: `postgres,redis,metrics`) |
| `--no-tidy` | `create` | skip `go mod tidy` |
| `--no-watch` | `run` | run once |
| `--fields "<list>"` | `make:model` | optional: columns to write in, e.g. `mobile:string,name:string` |
| `--table <name>` | `make:model` | table name (default: the plural, `category` → `categories`) |
| `--soft-delete` | `make:model` | add `deleted_at`; deletes set it and queries skip those rows |
| `--members "<list>"` | `make:enum` | `pending,paid=5,sent`: integers count up from 1 or the previous value; strings default to the name |
| `--type <type>` | `make:enum` | `string` or an integer type (default `int64`) |
| `--type <type>` | `make:cache` | the cached Go type, e.g. `int64`, `[]string`, `*User` (default `string`) |
| `--topic <topic>` | `make:worker`, `make:cdc` | topic consumed / published to |
| `--group <group>` | `make:worker` | consumer group (default: `<subject>.<name>`) |
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
	users := userapi.NewHandler(s.User)
	g.GET("/users", users.List)
	g.GET("/users/:id", users.Get)

	orders := orderapi.NewHandler(s.Order)
	g.GET("/orders", orders.List)
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

The struct is the only place a column is declared — like an ORM's tags, but read
**when you generate**, not at run time:

```go
// internal/domain/user/user.go
//gortisan:table users
type User struct {
	ID        int64      `db:"id,pk" json:"id"`
	Mobile    string     `db:"mobile" json:"mobile"`
	Name      string     `db:"name" json:"name"`
	Note      string     `json:"note"`                         // no db tag: not a column
	CreatedAt time.Time  `db:"created_at,default" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at,default" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at,default" json:"deleted_at,omitempty"`
}
```

`gortisan generate` writes the mapping at the end of the package's
`repository.go`, between two marker lines — plain code, with the columns, the
`Scan` and the insert list spelled out. Nothing else in the file is touched:

```go
// gortisan:table User begin — generated from user.go by gortisan generate; edits here are overwritten.
var userTable = orm.Table[User]{
	Name:       "users",
	PrimaryKey: "id",
	Columns:    []string{"id", "mobile", "name", "created_at", "updated_at", "deleted_at"},
	Scan: func(row orm.Scanner, m *User) error {
		return row.Scan(&m.ID, &m.Mobile, &m.Name, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt)
	},
	InsertColumns:    []string{"mobile", "name"},
	InsertValues:     func(m *User) []any { return []any{m.Mobile, m.Name} },
	UpdatedAtColumn:  "updated_at",
	SoftDeleteColumn: "deleted_at",
}
// gortisan:table User end
```

| In the struct | Means |
|---|---|
| `//gortisan:table name` | this struct is a table |
| `db:"name"` | a column, read and inserted |
| `db:"id,pk"` | the primary key: read, never inserted |
| `db:"created_at,default"` | filled by the database: read back after insert, never inserted |
| no `db` tag, or `db:"-"` | not a column |
| a column named `updated_at` / `deleted_at` | set to `now()` on update / soft delete |

To add a column, add a field and a migration. `make:model` and `gortisan run`
regenerate the mapping for you; in CI, `gortisan generate --check` fails when one
is out of date. Nothing inspects the struct at run time.

The `Service` depends on `Store`, an interface in `store.go` that `Repository`
implements — so a service test can pass a fake store and run without a database.
The repository returns the domain's `ErrNotFound`, never the orm's, so nothing
above it depends on the storage. `internal/platform/database/orm` lives in your
project; use it, write SQL by hand, or mix both:

```go
u, err := userTable.Find(ctx, r.db, id)
users, err := userTable.Get(ctx, r.db,
	orm.Where("name ILIKE ?", "%ali%").Where("mobile LIKE ?", "0912%").OrderBy("id DESC").Limit(20))

var n int64
err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE created_at >= $1`, since).Scan(&n)

err := database.InTx(ctx, db, func(tx pgx.Tx) error { // a transaction is a value, passed explicitly
	if err := users.WithTx(tx).Create(ctx, u); err != nil {
		return err
	}
	return payments.WithTx(tx).Create(ctx, p)
})
```

## Validation and errors

A request collects every problem and returns them together; what a check needs
is passed in like anything else:

```go
// internal/api/v1/userapi/request.go
func (r *CreateRequest) Validate(ctx context.Context, users *user.Service) error {
	var errs validate.Errors
	if len(r.Mobile) != 11 {
		errs.Add("mobile", "must be 11 digits")
	}
	if err := errs.Check(ctx, "mobile", user.NewMobileFree(users, r.Mobile)); err != nil {
		return err // the check could not run
	}
	return errs.Err()
}
```

```json
422 {"message": "The given data was invalid.",
     "errors": [{"field": "mobile", "message": "must be 11 digits"}]}
```

The domain returns errors in business terms (`errors.go`); the handler decides
the status:

```go
u, err := h.service.Find(ctx, req.ID)
if errors.Is(err, user.ErrNotFound) {
	return echo.NewHTTPError(http.StatusNotFound, "user not found")
}
if err != nil {
	return err // a 500; the details go to the log, not the client
}
```

Every error body is `{"message": "..."}`. Every request writes one JSON log line
with method, path, status and duration; 5xx lines are logged with their cause, and
a panic with its stack.

## Kafka: workers and CDC

`create --with kafka` adds `internal/platform/kafka` (readers, writers, `Publish`)
and `KAFKA_BROKERS`.

`make:worker user/send_welcome --topic users.created` writes
`internal/worker/userworker/worker.go` with a `Run(ctx)` loop: fetch, decode the
JSON message, handle, commit. A failed message is logged and committed so it
cannot block the partition — the loop is in your file, so change that if you need
retries. The first worker also writes `cmd/worker/main.go`; each later one prints
the `case` to add there. Run one process per worker:

```bash
./gortisan run worker -- user.send_welcome      # or: go run ./cmd/worker user.send_welcome
```

`make:cdc user` writes `internal/domain/user/cdc.go`, which publishes the user's
changes keyed by id through `internal/platform/cdc`:

```json
{"reference_id": 42, "log_time": "2026-11-19 10:00:00.000000+03:30", "operation": "U",
 "old_data": {"id": 42, "name": "Ali"}, "new_data": {"id": 42, "name": "Reza"},
 "change_data": ["name"]}
```

List the fields consumers should see in `CDCData`; an update that changes none of
them publishes nothing. Call `Created`, `Updated` and `Deleted` after the
transaction commits.

## Adding auth, Sentry, translations…

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
3. **Layers point one way:** `api`, `worker` → `domain` → `platform`. `internal/arch_test.go`
   enforces it in every project.
4. **No reflection in project code.** Config reads each variable by name; the table
   mappings are generated code. (echo's `c.Bind` is the one place a library uses
   reflection, and you can replace it.)
5. **Transactions are values** — `pgx.Tx` passed to `repo.WithTx(tx)` — and **errors
   carry meaning**: the domain names them, the API maps them to statuses.

## Not supported yet

- `migrate` / `migrate:rollback` / `migrate:status` and seeders (next)
- `make:test`, `route:list`, OpenAPI / Swagger docs
- running `create` again in an existing project to add a missing piece

## Development

```bash
go test ./...          # also creates a project for every --with choice, runs every
                       # make:* in it, then go mod tidy, go vet and go test (downloads modules)
go test -short ./...   # unit tests only
```

Templates live in `internal/generator/templates` and are embedded in the binary.
CI runs the full suite on Linux and Windows for every push and pull request.

Releases are git tags: `git tag -a v0.0.4 -m v0.0.4 && git push origin v0.0.4`.
