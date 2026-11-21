# Layout1

Created with gortisan. Every file here belongs to this
project: nothing imports gortisan, and gortisan is not in `go.mod`.

## Run

```bash
cp .env.example .env         # point it at your services
./gortisan run               # builds, runs `serve`, restarts on every change
                             # (or: go run . serve)
curl localhost:8000/ping     # {"ping":"pong"}
curl localhost:8000/health   # {"status":"pass","checks":{...}}
open  localhost:8000/swagger/index.html
```

Every feature is served under `BASE_PATH` (default `/api`), e.g. `/api/users`.

## Commands

```bash
go run . --help              # every command of the service
go run . serve               # the HTTP server
go run . migrate up          # apply pending migrations (also: down, status)
```

In production the image runs `layout1 serve`; with `RUN_MIGRATIONS=true`
`deploy/entrypoint.sh` runs `layout1 migrate up` first. Build it with
`docker build -f deploy/Dockerfile .`.

## Layout and rules

See [AGENTS.md](AGENTS.md): what each directory owns, what a feature package
holds, and the rules `tests/arch_test.go` enforces.

## Generate code

`./gortisan` (`gortisan.bat` or `.\gortisan.ps1` on Windows) runs the gortisan
version this project pins:

```bash
./gortisan make:feature user --fields "mobile:string,name:string"
./gortisan make:handler user/list
./gortisan route:list
./gortisan list              # every command
./gortisan upgrade           # pin the latest gortisan in the scripts
```

## Tests

```bash
go test ./...                # unit tests and the dependency rules
```

`internal/platform/orm` also has integration tests against a real database;
they run when `ORM_TEST_DSN` points at one and are skipped otherwise.
