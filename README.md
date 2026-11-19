# gortisan

[![test](https://github.com/alinemone/gortisan/actions/workflows/test.yml/badge.svg)](https://github.com/alinemone/gortisan/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alinemone/gortisan.svg)](https://pkg.go.dev/github.com/alinemone/gortisan)

A code generator for Go HTTP services on [echo](https://echo.labstack.com), in the
spirit of Laravel's `artisan` — without a framework behind it.

gortisan writes files and stops there. The project it creates **does not import
gortisan**, gortisan is **not in its `go.mod`**, and nothing runs at request time
that you cannot read in your own repository. Delete gortisan tomorrow and the
project still builds.

> **Status:** experimental (`v0.0.x`). Commands and templates may still change.

- [Install](#install)
- [Quick start](#quick-start)
- [Project layout](#project-layout)
- [The `./gortisan` script](#the-gortisan-script)
- [Commands](#commands)
- [The database layer](#the-database-layer)
- [Validation](#validation)
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
curl localhost:8080/metrics   # Prometheus
```

Add users, with a list endpoint:

```bash
./gortisan make:model user --fields "mobile:string,name:string" --soft-delete
./gortisan make:service user
./gortisan make:handler user/list
```

Each command prints what is left to wire by hand — a route in `router.go`, a
constructor call in `main.go`.

## Project layout

gortisan follows the layout the Go team describes for servers in
[Organizing a Go module](https://go.dev/doc/modules/layout#server-project) —
commands in `cmd/`, everything else in `internal/` — and the
[Google Go style guide](https://google.github.io/styleguide/go/best-practices#package-size)
on packages: code that is always used together belongs in one package, split
into focused files, and names do not repeat the package (`user.Repository`, not
`repository.UserRepository`).

So each subject is **one package**, not one package per layer:

```
shop/
├── cmd/
│   ├── shop/main.go            builds every dependency in order and starts the server
│   └── worker/main.go          runs one Kafka worker by name                    (kafka)
├── internal/
│   ├── user/                   everything about users
│   │   ├── user.go             type User — the table, as a struct with db tags
│   │   ├── status.go           type Status — an enum            (make:enum user/status)
│   │   ├── repository.go       type Repository — and the generated table mapping
│   │   ├── service.go          type Service — business logic
│   │   ├── handler.go          type Handler — HTTP endpoints
│   │   ├── request.go          ListRequest, GetRequest… — inputs, with Validate
│   │   ├── response.go         ListResponse, GetResponse… — outputs
│   │   ├── validator.go        reusable validation rules        (make:validator)
│   │   ├── cache.go  worker.go  cdc.go                          (when you make them)
│   ├── post/                   the next subject, the same files
│   ├── config/                 env vars (+ .env) → Config; no struct tags, no reflection
│   ├── server/                 echo: JSON request log, panic recovery, JSON errors, graceful shutdown
│   ├── router/                 every route, in one file
│   ├── health/                 /ping and /health
│   ├── validate/               field errors → one 422
│   ├── database/  database/orm/                                            (postgres)
│   ├── cache/                  the Redis client                             (redis)
│   ├── kafka/  cdc/                                                         (kafka)
│   └── metrics/                Prometheus                                   (metrics)
├── migrations/                 plain SQL, *.up.sql / *.down.sql             (postgres)
├── gortisan, gortisan.bat, gortisan.ps1
└── go.mod                      echo, pgx, go-redis, kafka-go, prometheus — only what you picked
```

Files that collect a kind of declaration — `handler.go`, `request.go`,
`response.go`, `validator.go`, `cache.go`, `worker.go` — are created by the first
command that needs them and **appended to** by the next: `make:handler user/get`
adds a method to `handler.go`, `GetRequest` to `request.go` and `GetResponse` to
`response.go`, and adds the imports they need. Nothing already in the file
changes; a name that already exists is left alone and reported.

A feature reads like this:

```go
// internal/user/handler.go
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	var req GetRequest // request.go
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(ctx); err != nil {
		return err
	}
	u, err := h.service.Find(ctx, req.ID)
	if errors.Is(err, orm.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if err != nil {
		return err // a 500; the details go to the log, not the client
	}
	return c.JSON(http.StatusOK, NewResponse(u)) // response.go
}
```

```go
// cmd/shop/main.go — wiring in plain sight
users := user.NewService(user.NewRepository(db))
handlers := router.Handlers{
	Health:  health.NewHandler(db, rdb),
	Metrics: m.Handler(cfg.MetricsSecret),
	User:    user.NewHandler(users),
}

// internal/router/router.go
v1.GET("/users", h.User.List)
v1.GET("/users/:id", h.User.Get)
```

Every error body is `{"message": "..."}`. Every request writes one JSON log line
with method, path, status and duration; 5xx lines are logged as errors with the
cause, and a panic is logged with its stack.

## The `./gortisan` script

Every project gets `gortisan`, `gortisan.bat` and `gortisan.ps1` in its root, like
`artisan` in Laravel and Goravel:

```bash
./gortisan list                 # macOS / Linux / Git Bash
gortisan.bat list               # cmd
.\gortisan.ps1 list             # PowerShell
```

Goravel's `artisan` runs `go run . artisan`, so the project has to import the
framework. gortisan's script runs

```
go run github.com/alinemone/gortisan/cmd/gortisan@<version> "$@"
```

pinned to the version that created the project. Everyone on the team generates
the same code, nobody needs gortisan installed, and the project's code and
`go.mod` stay untouched. Delete the three scripts if you do not want them.

## Commands

| Command | What it does |
|---|---|
| `create <name>` | a new project in `./<name>` |
| `run [command] [-- args]` | builds and runs `cmd/<command>`, rebuilding and restarting when a `.go` file, `go.mod` or `.env` changes; regenerates table mappings first; a failed build waits for the next change. `--no-watch` runs once |
| `generate` | writes the table mapping of every `//gortisan:table` struct into its `repository.go` (`make:model` and `run` do it for you); `--check` fails if one is out of date |
| `make:model <feature>` | `internal/<feature>/<feature>.go` (the struct), `repository.go`, a create-table migration *(postgres)* |
| `make:service <feature>` | `service.go`, wired to the repository when there is one |
| `make:handler <feature>/<action>` | a method in `handler.go`, its input in `request.go`, its output in `response.go` |
| `make:request <feature>/<name>` | a type with `Validate` in `request.go` |
| `make:response <feature>/<name>` | a type in `response.go` |
| `make:validator <feature>/<name>` | a reusable rule (`validate.Rule`) in `validator.go` |
| `make:enum <feature>/<name>` | `<name>.go`: constants, `String`, `Valid`, `Parse…`, the list of values |
| `make:cache <feature>/<name>` | a typed Redis cache around a loader in `cache.go` *(redis)* |
| `make:worker <feature>/<name>` | a Kafka consumer in `worker.go`, and `cmd/worker` *(kafka)* |
| `make:cdc <feature>` | `cdc.go`: change events of the model to Kafka *(kafka, postgres)* |
| `make:middleware <name>` | `internal/middleware/<name>.go`, an `echo.MiddlewareFunc` |
| `make:client <name>` | `internal/<name>/client.go`, a JSON HTTP client for another service |
| `make:migration <name>` | empty `migrations/<time>_<name>.{up,down}.sql` |
| `list`, `help`, `version` | |

`make handler` (with a space) is the same as `make:handler`. A kind marked
*(postgres)*, *(redis)* or *(kafka)* needs that piece in the project and says so
otherwise. A feature cannot take the name of a package gortisan generates
(`config`, `database`, `server`, …).

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
| `--topic <topic>` | `make:worker`, `make:cdc` | topic consumed / published to (default: the worker name / the table) |
| `--group <group>` | `make:worker` | consumer group (default: `<feature>.<name>`) |
| `--force` | `make:*` | overwrite whole files that exist |

**Field types** for `--fields`: `string`/`text`, `int`/`int64`/`bigint`,
`int32`/`integer`, `int16`, `float`/`float64`, `bool`, `time`, `json`/`jsonb`; a
trailing `?` makes the column nullable (a pointer in Go).

## The database layer

The struct is the only place a column is declared — like an ORM's tags, but read
**when you generate**, not at run time:

```go
// internal/user/user.go
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

To add a column, add a field and a migration. You rarely run `generate`
yourself: `make:model` runs it, and `gortisan run` runs it before every rebuild,
so adding a field and saving is enough. In CI, `gortisan generate --check` fails
when a mapping is out of date. Nothing inspects the struct at run time — there
is no reflection in the project.

`internal/database/orm` lives in your project. Use it, write SQL by hand, or mix
both on the same connection or transaction:

```go
// mapper
u, err := userTable.Find(ctx, r.db, id)
users, err := userTable.Get(ctx, r.db,
	orm.Where("name ILIKE ?", "%ali%").Where("mobile LIKE ?", "0912%").OrderBy("id DESC").Limit(20))
updated, err := userTable.Update(ctx, r.db, id, orm.Set{"name": "Reza"})

// plain SQL
var n int64
err := r.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE created_at >= $1`, since).Scan(&n)

// transaction: passed explicitly, never through the context
err := database.InTx(ctx, db, func(tx pgx.Tx) error {
	if err := users.WithTx(tx).Create(ctx, u); err != nil {
		return err
	}
	return posts.WithTx(tx).Create(ctx, p)
})
```

Arguments are always sent as parameters, never written into the SQL, and `Set`
rejects column names that are not in `Columns`. To drop the mapper, rewrite the
repository methods with SQL and delete `internal/database/orm`.

## Validation

A request collects every problem and returns them together:

```go
// internal/user/request.go — what a check needs is passed in, like anything else
func (r *CreateRequest) Validate(ctx context.Context, users *Repository) error {
	var errs validate.Errors
	if len(r.Mobile) != 11 {
		errs.Add("mobile", "must be 11 digits")
	}
	// a reusable rule from `make:validator user/mobile_free`;
	// an error means the check could not run
	if err := errs.Check(ctx, "mobile", NewMobileFree(users, r.Mobile)); err != nil {
		return err
	}
	return errs.Err()
}
```

```json
422 {"message": "The given data was invalid.",
     "errors": [{"field": "mobile", "message": "must be 11 digits"}]}
```

## Kafka: workers and CDC

`create --with kafka` adds `internal/kafka` (readers, writers, `Publish`) and
`KAFKA_BROKERS`.

`make:worker user/send_welcome --topic users.created` adds `SendWelcomeWorker`
to `internal/user/worker.go`, with a `Run(ctx)` loop: fetch, decode the JSON
message, handle, commit. A failed message is logged and committed so it cannot
block the partition — the loop is in your file, so change that if you need
retries. The first worker also writes `cmd/worker/main.go`; each later one
prints the `case` to add there. Run one process per worker:

```bash
./gortisan run worker -- user.send_welcome      # or: go run ./cmd/worker user.send_welcome
```

`make:cdc user` writes `internal/user/cdc.go`, which publishes the user's changes
keyed by id (through the shared `internal/cdc`):

```json
{"reference_id": 42, "log_time": "2026-11-19 10:00:00.000000+03:30", "operation": "U",
 "old_data": {"id": 42, "name": "Ali"}, "new_data": {"id": 42, "name": "Reza"},
 "change_data": ["name"]}
```

List the fields consumers should see in `CDCData`; an update that changes none
of them publishes nothing. Call `Created`, `Updated` and `Deleted` after the
transaction commits.

## Adding auth, Sentry, translations…

gortisan does not generate them, on purpose: every company does them
differently, and a generated version would have to pick a library for you. They
are a few lines each, added by hand, and they stay explicit — a middleware and a
constructor call in `main.go`:

```bash
./gortisan make:middleware authenticate
```

```go
// internal/middleware/authenticate.go
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

// internal/router/router.go
v1.Use(middleware.Authenticate(h.Tokens))
```

Error reporting (Sentry and the like) goes in the same way: a middleware passed to
`server.New(logger, sentryMiddleware)`, or a call in `server`'s error handler.

## No magic: the rules

The generated code follows these rules, and gortisan's own tests check the first one
on every build:

1. **No import of the generator.** Only the project's own packages and the
   libraries you picked (echo, pgx, go-redis, kafka-go, prometheus).
2. **Dependencies are passed, not looked up.** Constructors take what they need and
   `main.go` wires them; nothing is stored in or resolved from `context.Context`.
3. **No reflection in project code.** Config reads each variable by name; the
   table mappings are generated code that scans the columns it lists. (echo's
   `c.Bind` is the one place a library uses reflection, and you can replace it.)
4. **Transactions are values.** `pgx.Tx` is passed to `repo.WithTx(tx)`.
5. **Errors carry their status.** `echo.NewHTTPError(code, msg)` for client errors;
   anything else is a 500 that is logged and not shown to the client.

## Not supported yet

- OpenAPI / Swagger docs
- running `create` again in an existing project to add a missing piece
- overriding the built-in templates per project

## Development

```bash
go test ./...          # also creates a project for every --with choice, runs every
                       # make:* in it, then go mod tidy, go vet and go test (downloads modules)
go test -short ./...   # unit tests only
```

Templates live in `internal/generator/templates` and are embedded in the binary.
CI runs the full suite on Linux and Windows for every push and pull request.

Releases are git tags: `git tag -a v0.0.4 -m v0.0.4 && git push origin v0.0.4`.
