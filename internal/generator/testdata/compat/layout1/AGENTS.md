# Layout1 — rules for people and coding agents

Feature-first Go service: Cobra commands in `cmd/`, one package per feature in
`internal/`, wiring in `internal/app`, shared code in `internal/server`,
`internal/platform`, `internal/clients` and `internal/external`.
`tests/arch_test.go` enforces the dependency rules on every `go test ./...`.

## Layout

```
main.go                  calls cmd.Execute(); general Swagger info (@title, @version, @BasePath)
cmd/                     Cobra commands only: arguments, then a call into internal/app
  root.go                every command of the service, listed in Execute
  serve.go               layout1 serve
  migrate.go             layout1 migrate up | down | status
docs/                    written by swag (`./gortisan swag`) — never edit by hand
internal/
  app/                   wiring: connections -> features -> routes (serve.go, migrate.go)
  config/                the Config struct, read from the environment and .env
  <feature>/             one package per feature (user, sales/order, ...)
  server/                echo, middleware/, response/ (RFC 9457 errors), validate/, validator/
  platform/              connections to infrastructure: postgres.go, orm/, redis.go, trans/
  clients/<name>/        HTTP/gRPC clients for our own services
  external/<name>/       clients for third-party APIs
  migrations/            SQL migrations, embedded in the binary (goose)
tests/arch_test.go       the dependency rules
deploy/                  Dockerfile and entrypoint.sh
```

## What each directory owns

| Path | Owns | Must not |
|---|---|---|
| `main.go` | Calling `cmd.Execute()`; general Swagger info | Hold any logic |
| `cmd/` | Cobra commands and their arguments | Build dependencies; it hands over to `internal/app` |
| `internal/app/` | Opening connections, building repositories → services → handlers, mounting routes | Contain business rules |
| `internal/config/` | The `Config` struct and reading it from the environment | — (nothing else reads environment variables) |
| `internal/<feature>/` | Everything of one feature: entity, errors, use cases, storage, HTTP, DTOs, routes, tests | Import another feature |
| `internal/server/` | Echo, shared middleware, error responses, validation | Know about a feature |
| `internal/platform/` | Connections to Postgres, Redis, … and translations | Contain feature queries |
| `internal/clients/`, `internal/external/` | One client package per service or API | Leak their transport types into features |
| `internal/migrations/` | Numbered SQL files run by `layout1 migrate` | Change a migration that already ran; add a new one |
| `docs/` | The output of swag | Be edited by hand; change the comments and run `./gortisan swag` |

## A feature package

| File | Contents |
|---|---|
| `<feature>.go` | The entity and the feature's errors, e.g. `ErrNotFound` |
| `service.go` | `Service`, its use cases, and the `Repository` interface it needs |
| `postgres.go` | `Postgres`, the `Repository` on the database (orm over bun) |
| `handler.go` | Echo handlers with swag comments: bind, validate, call the service, write a DTO |
| `dto.go` | Request and response types |
| `routes.go` | `RegisterRoutes(api *echo.Group, h *Handler)`: paths relative to `BASE_PATH` |
| `service_test.go` | Table-driven tests of the service with a fake repository |

## Adding to the service

Use the generator; it writes the files where they belong and wires them:

```bash
./gortisan make:feature user --fields "mobile:string,name:string"   # the package, wired into internal/app
./gortisan make:handler user/list        # handler + ListRequest/ListResponse + the route
./gortisan make:migration add_index_to_users
./gortisan make:enum user/status --members "active,blocked"
./gortisan make:validator address/mobile
./gortisan make:client wallet            # internal/clients/wallet
./gortisan make:external bale            # internal/external/bale
./gortisan make:test user/list
./gortisan route:list
./gortisan list                          # every command
```

After a change: `go test ./...` (includes the dependency rules), `gofmt`, `go vet ./...`. After changing a handler's swag comments: `./gortisan swag`.

## Rules

- Interfaces are declared by the code that uses them, small, and unexported where possible.
- No global state: everything is passed through constructors, built in `internal/app`.
- One feature calls another through a small interface it declares; `internal/app` passes the other feature in.
- Handlers return errors; `internal/server` is the one place that turns an error into a status and an RFC 9457 body.
- Texts shown to people are keys of `internal/platform/trans/lang/fa.json`.
- Secrets come from environment variables only; `.env.example` lists the names, never real values.
