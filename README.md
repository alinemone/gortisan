# gortisan

A code generator for Go services on [echo](https://echo.labstack.com). It writes
files and stops there: the generated project does not import gortisan, gortisan
is not in its `go.mod`, and nothing runs at request time that you cannot read in
your own repository.

```bash
go install github.com/alinemone/gortisan/cmd/gortisan@latest
```

## Create a project

```bash
gortisan create shop --module github.com/you/shop
cd shop && cp .env.example .env
go run ./cmd/shop          # /ping, /health
```

`--with postgres,redis` picks the pieces (default `all`; also `none`).
`--no-tidy` skips `go mod tidy`.

```
cmd/shop/main.go                 builds every dependency and starts the server
internal/config                  env vars (+ .env) -> Config, no tags, no reflection
internal/server                  echo: request log (slog), panic recovery, JSON errors, graceful shutdown
internal/router                  every route in one file
internal/handler                 HTTP handlers
internal/service                 business logic
internal/database                pgx pool, DBTX, InTx            (postgres)
internal/database/orm            the table mapper, part of the project (postgres)
internal/model, internal/repository, migrations                  (postgres)
internal/cache                   go-redis client                  (redis)
```

The only libraries are echo, pgx and go-redis.

## Add files

Like Laravel's and Goravel's `artisan`, every project gets a `./gortisan` script in
its root (`gortisan.bat` and `gortisan.ps1` on Windows):

```bash
./gortisan list
./gortisan make:handler order/get_order      # internal/handler/order.go, order_get_order.go
./gortisan make:service order                # internal/service/order_service.go
./gortisan make:model order --fields "user_id:int64,status:string,paid_at:time?" --soft-delete
./gortisan make:migration add_index_to_orders
```

Goravel's `artisan` is `go run . artisan`, so the project imports the framework.
gortisan's script is `go run github.com/alinemone/gortisan/cmd/gortisan@<version>`
instead: the version that created the project, run from the module cache, with
nothing imported and nothing added to `go.mod`. The global `gortisan` binary takes
the same commands, and `make handler` (with a space) works as well as `make:handler`.

`make model` writes the struct, a repository with its `orm.Table` mapping, and
`migrations/<time>_create_<table>_table.{up,down}.sql`. Field types: `string`,
`text`, `int`, `int64`, `bigint`, `int32`, `integer`, `int16`, `float`, `float64`,
`bool`, `time`, `json`, `jsonb`; a trailing `?` makes the column nullable. Every
model gets `id`, `created_at` and `updated_at`.

Existing files are never overwritten unless you pass `--force`. Each command
prints what to wire by hand (a route, a constructor in `main.go`); gortisan does
not edit code you own.

## What "no magic" means here

| framework-style generator | gortisan output |
|---|---|
| `di.Resolve[T](ctx)` reads dependencies from the request context | constructors take their dependencies; `main.go` passes them |
| `state.Get[T](ctx)` per-request values keyed by `reflect.Type` | plain variables and parameters |
| `handler.Handle` lifecycle + reflective `Bind` | a method `func (h *OrderHandler) GetOrder(c echo.Context) error` |
| `postgres.Repository[T]` (bun + reflect), transaction carried in `ctx` | `orm.Table[T]` in the project (listed columns, explicit `Scan`), transaction passed as `pgx.Tx` |
| generated code imports the generator's own packages | imports only the project's own packages and echo/pgx/go-redis |

## Develop

```bash
go test ./...          # builds every --with variant end to end (downloads modules)
go test -short ./...   # unit tests only
```

Templates live in `internal/generator/templates` and are embedded in the binary.
