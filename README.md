# gortisan

**English** | [فارسی](README.fa.md)

[![test](https://github.com/alinemone/gortisan/actions/workflows/test.yml/badge.svg)](https://github.com/alinemone/gortisan/actions/workflows/test.yml)

A code generator for Go HTTP services on [echo v5](https://echo.labstack.com), in
the spirit of Laravel's `artisan` — without a framework behind it.

One feature-first layout for every service of the team: Cobra commands in
`cmd/`, one package per feature in `internal/`, wiring in `internal/app`.
gortisan writes the files, wires them, and stops there: the project **does not
import gortisan**, gortisan is **not in its `go.mod`**, and nothing runs at
request time that you cannot read in your own repository.

> **Status:** preview (`v0.x`). From `v1.0.0` every release keeps existing
> projects working — see [COMPATIBILITY.md](COMPATIBILITY.md).

- [Install](#install)
- [Quick start](#quick-start)
- [Project layout](#project-layout)
- [Commands](#commands)
- [Features](#features)
- [Migrations](#migrations)
- [Swagger](#swagger)
- [The database layer](#the-database-layer)
- [Validation and errors](#validation-and-errors)
- [More commands: workers, schedules](#more-commands-workers-schedules)
- [Versions and upgrades](#versions-and-upgrades)
- [No magic: the rules](#no-magic-the-rules)
- [Development](#development)

## Install

Requires Go 1.26 or newer.

```bash
go install github.com/alinemone/gortisan/cmd/gortisan@latest
gortisan version
```

### If Go cannot find gortisan

gortisan is a private module on the company GitLab. When `go install` or a
project's `./gortisan` says the module is not found (`404`, `unknown
revision`, `terminal prompts disabled`), set the company Go settings once:

```bash
go env -w GOPRIVATE=git.basalam.dev
go env -w GOPROXY=https://repo.basalam.dev/artifactory/api/go/basalam-golang-local,https://proxy.golang.org,direct
```

and let Go reach GitLab over SSH, with the key you already use for git:

```bash
git config --global url."git@git.basalam.dev:".insteadOf "https://git.basalam.dev/"
```

`GOPRIVATE` makes Go fetch `git.basalam.dev` modules from git directly, and
skip the public checksum database. Check with `go env GOPRIVATE GOPROXY`.

## Quick start

```bash
gortisan create shop github.com/you/shop   # or: gortisan create github.com/you/shop
cd shop
cp .env.example .env                       # point it at PostgreSQL and Redis
./gortisan make:feature user --fields "mobile:string,name:string"
./gortisan make:handler user/list
go run . migrate up
./gortisan run                             # go run . serve, restarted on every change
curl localhost:8000/api/users
```

`make:feature` writes the feature package and wires it into
`internal/app/serve.go`; `make:handler` writes the handler, its request and
response, and its route. Each command prints anything left to do by hand.

A service whose **database already exists** leaves migrations out:

```bash
gortisan create shop --with postgres,redis,swagger
```

`--with` picks from `postgres`, `redis`, `migrations` (needs postgres) and
`swagger`; the default is all of them. `gortisan add <component>` adds one later.

## Project layout

```
shop/
├── main.go                      cmd.Execute(); the general Swagger info
├── cmd/                         Cobra commands only: arguments, then internal/app
│   ├── root.go                  every command of the service, in one list
│   ├── serve.go                 shop serve
│   └── migrate.go               shop migrate up | down | status
├── docs/                        written by swag — never edited by hand
├── internal/
│   ├── app/                     wiring: connections → features → routes
│   │   ├── serve.go
│   │   └── migrate.go
│   ├── config/                  Config, read from the environment and .env
│   ├── user/                    one package per feature (make:feature user)
│   │   ├── user.go              entity + errors
│   │   ├── service.go           use cases + the Repository interface they need
│   │   ├── postgres.go          the Repository on the database
│   │   ├── handler.go           echo handlers, with swag comments
│   │   ├── dto.go               requests and responses
│   │   ├── routes.go            RegisterRoutes(api, h): paths under BASE_PATH
│   │   └── service_test.go      table-driven, with a fake Repository
│   ├── sales/order/             features can be grouped
│   ├── server/                  echo, middleware/, response/ (RFC 9457), validate/, validator/
│   ├── platform/                postgres.go, redis.go, orm/, trans/
│   ├── clients/wallet/          clients for our own services   (make:client)
│   ├── external/bale/           clients for third-party APIs   (make:external)
│   └── migrations/              SQL migrations, embedded (goose)
├── tests/arch_test.go           the dependency rules, run by go test ./...
├── deploy/                      Dockerfile, entrypoint.sh
├── AGENTS.md, CLAUDE.md, .agents/skills/   rules and skills for coding agents
├── .gortisan.json               the layout and components, for later gortisan versions
└── gortisan, gortisan.bat, gortisan.ps1
```

**Dependency direction:** `main.go → cmd → internal/app → feature (handler → service → Repository ← postgres)`.
Only `internal/app` knows the concrete types. Features use `server`,
`platform`, `clients` and `external`; those never import a feature, and a
feature never imports another one — it declares a small interface, and
`internal/app` passes the other feature in. `tests/arch_test.go` fails the
build when a rule is broken.

**Routes:** every feature is mounted under `BASE_PATH` (default `/api`), and
each feature's `routes.go` sets its own path relative to it, e.g.
`api.Group("/users")` → `/api/users`. A feature that needs a version puts it
in its path (`/v2/users`). `/ping`, `/health` and `/swagger/*` are outside
`BASE_PATH`.

## Commands

| Command | What it does |
|---|---|
| `create <name> [<module>]` | a new project in `./<name>`; `--with`, `--no-tidy` |
| `make:feature <name>` | a feature package, wired into `internal/app`; `--fields`, `--table`, `--soft-delete`; with migrations, its create-table migration |
| `make:handler <feature>/<action>` | a handler method (with swag comments), its `<Action>Request` and `<Action>Response` in `dto.go`, and its route in `routes.go` |
| `make:migration <name>` | an empty `internal/migrations/<time>_<name>.sql` |
| `make:enum <feature>/<name>` | an enum: constants, `String`, `Valid`, `Parse…`; `--members`, `--type` |
| `make:cache <feature>/<name>` | a typed Redis cache in the feature's `cache.go`; `--type` |
| `make:validator [<group>/]<name>` | a reusable validator in `internal/server/validator`, and its message key in `fa.json` |
| `make:middleware <name>` | `internal/server/middleware/<name>.go` |
| `make:client <name>` / `make:external <name>` | an HTTP client in `internal/clients/<name>` / `internal/external/<name>` |
| `make:test <feature>[/<action>]` | a service test, or an HTTP test of one action |
| `add <component>` | add postgres, redis, migrations or swagger to an existing project |
| `run [command] [-- args]` | build and run `serve` (or another command), restarting on every change |
| `route:list` | every route: method, path, handler, file:line |
| `swag` | regenerate `docs/` with the pinned swag — nothing to install |
| `upgrade [version]` | pin another gortisan version in the project |
| `version`, `completion`, `help` | |

`make feature user` (with a space) works too. `--force` overwrites whole files
that make writes; without it existing files are kept.

Inside a project, use the `./gortisan` script (`gortisan.bat`, `.\gortisan.ps1`
on Windows): it runs, through `go run`, the gortisan version the project pins,
so everyone generates the same code and nobody needs gortisan installed.

## Features

```bash
./gortisan make:feature sales/order --fields "user_id:int64,amount:int64,paid_at:time?" --soft-delete
./gortisan make:handler sales/order/create    # POST /api/sales/orders
./gortisan make:handler sales/order/get       # GET  /api/sales/orders/:id
```

The action name picks the route: `list` and `create` go to the collection,
`get`, `update`, `patch` and `delete` to `/:id`, any other name to
`/<action>` (`mark_paid` → `GET /mark-paid`). Change it in `routes.go` freely;
later `make:handler`s read the group path from there.

Field types: `string`, `int`, `int32`, `int16`, `float`, `bool`, `time`,
`json`; a trailing `?` makes the column nullable (a pointer in Go).

## Migrations

```bash
./gortisan make:migration add_index_to_users   # internal/migrations/<time>_add_index_to_users.sql
go run . migrate up                            # also: down (the last one), status
```

A migration is one SQL file with an Up and a Down part, run by
[goose](https://github.com/pressly/goose):

```sql
-- +goose Up
CREATE INDEX users_mobile ON users (mobile);

-- +goose Down
DROP INDEX users_mobile;
```

The files are embedded in the binary. A Postgres advisory lock lets one
process migrate at a time. In production, `deploy/entrypoint.sh` runs
`migrate up` before the command when `RUN_MIGRATIONS=true`. Versions are
timestamps, so two branches never pick the same number.

## Swagger

Every handler `make:handler` writes carries swag comments; `main.go` carries the
general API info. Regenerate the document and open the UI:

```bash
./gortisan swag              # go run github.com/swaggo/swag/cmd/swag@<pinned> init
open http://localhost:8000/swagger/index.html
```

It is served by [echo-swagger](https://github.com/swaggo/echo-swagger) v2,
which supports echo v5.

## The database layer

PostgreSQL is reached through [bun](https://bun.uptrace.dev) over
[pgx](https://github.com/jackc/pgx). The entity is a plain struct with bun tags:

```go
// internal/user/user.go
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        int64      `bun:"id,pk,autoincrement" json:"id"`
	Mobile    string     `bun:"mobile" json:"mobile"`
	CreatedAt time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}
```

`internal/platform/orm` is a small layer over bun that lives in your project and
imports only bun, pgx and the standard library: `Repository[T]` (`Find`, `Get`,
`First`, `Count`, `Exists`, `Paginate`, `Create`, `Update`, `Delete`,
`ForceDelete`, `Restore`), scopes (`Where`, `OrderBy`, `Limit`, `WithTrashed`,
`ForUpdate`, …), relations (`With("Items.Statuses")`), pagination and explicit
transactions:

```go
err := orm.InTx(ctx, db, func(ctx context.Context, tx orm.IDB) error {
	if err := orders.WithTx(tx).Create(ctx, o); err != nil {
		return err // rolls back
	}
	return payments.WithTx(tx).Create(ctx, p) // nil commits
})
```

The service depends on its `Repository` interface, so `service_test.go` runs
with a fake one and no database. `postgres.go` returns the feature's
`ErrNotFound`, never the orm's.

## Validation and errors

A request validates itself after binding, and reports every invalid field as
one 422:

```go
func (r *CreateRequest) Validate(ctx context.Context) error {
	var errs validate.Errors
	if err := errs.Check(ctx, address.NewMobile("body.mobile", r.Mobile)); err != nil {
		return err
	}
	if r.Amount <= 0 {
		errs.Add("body.amount", "general:min", "مبلغ", 1)
	}
	return errs.Err()
}
```

`make:validator address/mobile` writes a reusable check in
`internal/server/validator/address/`; you write its `Passes`.

Every error — validation, `echo.NewHTTPError`, a missing route, a 500 — is
written by `internal/server` as [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)
problem details (`application/problem+json`). Texts are keys of
`internal/platform/trans/lang/fa.json`; a test fails when the code uses a key
the file does not have. The feature returns errors in business terms; the
handler picks the status.

## More commands: workers, schedules

A new process is a new Cobra command, next to `serve`:

```
cmd/worker.go          shop worker    → internal/app/worker.go (builds the consumers)
cmd/schedule.go        shop schedule  → internal/app/schedule.go
internal/order/consumer.go            a consumer calls order.Service, like a handler does
```

and one line in `cmd/root.go`. One image runs every role: `serve`, `worker`
or `schedule` as its command.

## Versions and upgrades

Each project pins its gortisan version in `./gortisan` and records its layout
in `.gortisan.json`. From `v1.0.0`, every release of the same major keeps
existing projects working; `./gortisan upgrade` moves a project to the newest
one. The full promise is in [COMPATIBILITY.md](COMPATIBILITY.md).

## No magic: the rules

1. **No import of the generator.** Only the project's packages and the libraries
   you picked. gortisan's tests check it on every build.
2. **Dependencies are passed, not looked up.** Constructors take what they need;
   `internal/app` builds them. Nothing is resolved from a context or a registry.
3. **Features stay apart.** A feature never imports another; shared code never
   imports a feature. `tests/arch_test.go` enforces it.
4. **Readable code over machinery.** Config reads each variable by name. The
   database goes through bun, and the thin `orm` on top is in your repository.
5. **Errors carry meaning:** the feature names them, the handler maps them to
   statuses, `internal/server` writes them.

A generated project depends on echo v5 and Cobra, and on what you pick: bun +
pgx, go-redis, goose, echo-swagger. Kafka, metrics, Sentry or tracing are yours
to add — a client in `platform/`, a middleware, a line in `internal/app`.

## Development

```bash
go test ./...          # also creates a project for several --with choices, runs every
                       # make:* in it, then go mod tidy, go vet and go test (downloads modules),
                       # and does the same on the compatibility fixtures
go test -short ./...   # unit tests only
```

Templates live in `internal/generator/templates` and are embedded in the binary.
Releases are git tags: `git tag -a v0.1.0 -m v0.1.0 && git push origin v0.1.0`.
