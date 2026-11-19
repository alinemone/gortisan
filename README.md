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
- [The `./gortisan` script](#the-gortisan-script)
- [Commands](#commands)
- [What gets generated](#what-gets-generated)
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

Add an endpoint backed by a table:

```bash
./gortisan make:model order --fields "user_id:int64,status:string,paid_at:time?" --soft-delete
./gortisan make:service order
./gortisan make:handler order/get_order
```

Each command prints what is left to wire by hand — a route in `router.go`, a
constructor call in `main.go`. gortisan never edits a file you already have.

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
| `run [command] [-- args]` | builds and runs `cmd/<command>`, rebuilding and restarting when a `.go` file, `go.mod` or `.env` changes; a failed build waits for the next change. `--no-watch` runs once |
| `make:handler <group>/<action>` | `internal/handler/<group>.go` (once) and `<group>_<action>.go`, with its request, validation and response |
| `make:service <name>` | `internal/service/<name>_service.go` |
| `make:model <name>` | model, repository with its table mapping, and a create-table migration *(postgres)* |
| `make:migration <name>` | empty `migrations/<time>_<name>.{up,down}.sql` |
| `make:middleware <name>` | `internal/middleware/<name>.go`, an `echo.MiddlewareFunc` |
| `make:request [<group>/]<name>` | `internal/request/<name>.go`, a request type with `Validate` |
| `make:response [<group>/]<name>` | `internal/response/<name>.go` |
| `make:validator [<group>/]<name>` | `internal/validator/<name>.go`, a reusable rule (`validate.Rule`) |
| `make:enum <name>` | `internal/enum/<name>.go`: constants, `String`, `Valid`, `Parse…`, the list of values |
| `make:cache <name>` | `internal/cache/<name>.go`, a typed Redis cache around a loader *(redis)* |
| `make:client <name>` | `internal/client/<name>/client.go`, a JSON HTTP client for another service |
| `make:worker <name>` | `internal/worker/<name>.go`, a Kafka consumer, and `cmd/worker` *(kafka)* |
| `make:cdc <model>` | `internal/cdc/<model>.go`, change events of a model to Kafka *(kafka, postgres)* |
| `list`, `help`, `version` | |

`make handler` (with a space) is the same as `make:handler`. A kind marked
*(postgres)*, *(redis)* or *(kafka)* needs that piece in the project and says so
otherwise.

**Flags**

| Flag | Command | Meaning |
|---|---|---|
| `--module <path>` | `create` | Go module path (default: the project name) |
| `--with <list>` | `create` | any of `postgres`, `redis`, `kafka`, `metrics`; `all`; `none` (default: `postgres,redis,metrics`) |
| `--no-tidy` | `create` | skip `go mod tidy` |
| `--no-watch` | `run` | run once |
| `--fields "<list>"` | `make:model` | columns, e.g. `user_id:int64,status:string,paid_at:time?` |
| `--table <name>` | `make:model` | table name (default: the plural, `category` → `categories`) |
| `--soft-delete` | `make:model` | add `deleted_at`; deletes set it and queries skip those rows |
| `--members "<list>"` | `make:enum` | `pending,paid=5,sent`: integers count up from 1 or the previous value; strings default to the name |
| `--type <type>` | `make:enum` | `string` or an integer type (default `int64`) |
| `--type <type>` | `make:cache` | the cached Go type, e.g. `int64`, `[]string`, `*model.Order` (default `string`) |
| `--topic <topic>` | `make:worker`, `make:cdc` | topic consumed / published to (default: the worker name / the table) |
| `--group <group>` | `make:worker` | consumer group (default: the worker name) |
| `--force` | `make:*` | overwrite files that exist |

**Field types** for `--fields`: `string`/`text`, `int`/`int64`/`bigint`,
`int32`/`integer`, `int16`, `float`/`float64`, `bool`, `time`, `json`/`jsonb`. A
trailing `?` makes the column nullable (a pointer in Go). Every model also gets
`id`, `created_at` and `updated_at`.

## What gets generated

```
shop/
├── cmd/shop/main.go           builds every dependency in order and starts the server
├── cmd/worker/main.go         runs one Kafka worker by name                (kafka)
├── internal/
│   ├── config/                env vars (+ .env) → Config; no struct tags, no reflection
│   ├── server/                echo: JSON request log (slog), panic recovery, JSON errors, graceful shutdown
│   ├── router/                every route, in one file
│   ├── handler/               HTTP handlers; health.go answers /ping and /health
│   ├── service/               business logic
│   ├── validate/              field errors → one 422
│   ├── database/  database/orm/  model/  repository/                       (postgres)
│   ├── cache/                 go-redis client and typed caches             (redis)
│   ├── kafka/  worker/  cdc/                                               (kafka)
│   ├── metrics/               Prometheus: requests by route and status, Go runtime, DB pool (metrics)
│   └── middleware/  request/  response/  validator/  enum/  client/        (when you make them)
├── migrations/                plain SQL, *.up.sql / *.down.sql             (postgres)
├── gortisan, gortisan.bat, gortisan.ps1
├── .env.example  Dockerfile  Makefile  README.md
└── go.mod                     echo, pgx, go-redis, kafka-go, prometheus — only what you picked
```

A handler is an ordinary method; its dependencies are fields set by its
constructor:

```go
type OrderHandler struct {
	orders *service.OrderService
}

func NewOrderHandler(orders *service.OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

func (h *OrderHandler) GetOrder(c echo.Context) error {
	ctx := c.Request().Context()
	var req GetOrderRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(ctx); err != nil {
		return err
	}
	order, err := h.orders.Find(ctx, req.ID)
	if errors.Is(err, orm.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}
	if err != nil {
		return err // a 500; the details go to the log, not the client
	}
	return c.JSON(http.StatusOK, order)
}
```

`main.go` builds it in plain sight:

```go
orders := repository.NewOrderRepository(db)
handlers := router.Handlers{
	Health:  handler.NewHealthHandler(db, rdb),
	Metrics: m.Handler(cfg.MetricsSecret),
	Order:   handler.NewOrderHandler(service.NewOrderService(orders)),
}
```

Every error body is `{"message": "..."}`. Every request writes one JSON log line
with method, path, status and duration; 5xx lines are logged as errors with the
cause, and a panic is logged with its stack.

## The database layer

`make:model` writes the mapping between the struct and the table into the
repository file, where you can read and change it:

```go
var orderTable = orm.Table[model.Order]{
	Name:       "orders",
	PrimaryKey: "id",
	Columns:    []string{"id", "user_id", "status", "paid_at", "created_at", "updated_at", "deleted_at"},
	Scan: func(row orm.Scanner, m *model.Order) error {
		return row.Scan(&m.ID, &m.UserID, &m.Status, &m.PaidAt, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt)
	},
	InsertColumns:    []string{"user_id", "status", "paid_at"},
	InsertValues:     func(m *model.Order) []any { return []any{m.UserID, m.Status, m.PaidAt} },
	UpdatedAtColumn:  "updated_at",
	SoftDeleteColumn: "deleted_at",
}
```

`internal/database/orm` lives in your project. Use it, write SQL by hand, or mix
both on the same connection or transaction:

```go
// mapper
order, err := orderTable.Find(ctx, r.db, id)
orders, err := orderTable.Get(ctx, r.db,
	orm.Where("user_id = ?", userID).Where("status IN (?, ?)", "paid", "sent").OrderBy("id DESC").Limit(20))
updated, err := orderTable.Update(ctx, r.db, id, orm.Set{"status": "paid"})

// plain SQL
var n int64
err := r.db.QueryRow(ctx, `SELECT count(*) FROM orders WHERE status = $1`, status).Scan(&n)

// transaction: passed explicitly, never through the context
err := database.InTx(ctx, db, func(tx pgx.Tx) error {
	if err := orders.WithTx(tx).Create(ctx, order); err != nil {
		return err
	}
	return items.WithTx(tx).Create(ctx, item)
})
```

Arguments are always sent as parameters, never written into the SQL, and `Set`
rejects column names that are not in `Columns`. To drop the mapper, rewrite the
repository methods with SQL and delete `internal/database/orm`.

## Validation

A request collects every problem and returns them together:

```go
func (r *CreateOrderRequest) Validate(ctx context.Context) error {
	var errs validate.Errors
	if r.Quantity <= 0 {
		errs.Add("quantity", "must be positive")
	}
	// a reusable rule from `make:validator`; an error means the check could not run
	if err := errs.Check(ctx, "order_id", validator.NewOrderNotPaid(r.orders, r.OrderID)); err != nil {
		return err
	}
	return errs.Err()
}
```

```json
422 {"message": "The given data was invalid.",
     "errors": [{"field": "quantity", "message": "must be positive"}]}
```

## Kafka: workers and CDC

`create --with kafka` adds `internal/kafka` (readers, writers, `Publish`) and
`KAFKA_BROKERS`.

`make:worker send_notifications --topic notifications` writes a consumer with a
`Run(ctx)` loop: fetch, decode the JSON message, handle, commit. A failed message
is logged and committed so it cannot block the partition — the loop is in your
file, so change that if you need retries. The first worker also writes
`cmd/worker/main.go`; each later one prints the `case` to add there. Run one
process per worker:

```bash
./gortisan run worker -- send_notifications      # or: go run ./cmd/worker send_notifications
```

`make:cdc order` publishes the order's changes, keyed by id:

```json
{"reference_id": 42, "log_time": "2026-11-19 10:00:00.000000+03:30", "operation": "U",
 "old_data": {"id": 42, "status": "new"}, "new_data": {"id": 42, "status": "paid"},
 "change_data": ["status"]}
```

List the fields consumers should see in `OrderCDCData`; an update that changes
none of them publishes nothing. Call `Created`, `Updated` and `Deleted` after the
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
			user, err := tokens.Verify(c.Request().Context(), c.Request().Header.Get("Authorization"))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			c.Set("user", user) // or pass it on explicitly
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
   mapper scans the columns it lists. (echo's `c.Bind` is the one place a library
   uses reflection, and you can replace it.)
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

Releases are git tags: `git tag -a v0.0.2 -m v0.0.2 && git push origin v0.0.2`.
