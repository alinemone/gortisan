# Compatibility

**English** | [فارسی](COMPATIBILITY.fa.md)

gortisan follows [semantic versioning](https://semver.org) from **v1.0.0**.
Releases before it (`v0.x`) are previews for the team to try the layout;
they may still change it.

## What a version promises

Within one major version (every `v1.x.y`):

1. **Commands and flags stay.** No command or flag is removed or renamed. New
   ones may be added. One that is replaced keeps working and prints a
   deprecation notice until the next major version.
2. **Every project keeps working with every newer gortisan of its major.**
   `make:*`, `add`, `route:list` and `run` on a project created by `v1.0.0`
   write code that fits that project, under any `v1.x.y`.
3. **The layout does not move.** Where a feature, a migration, a client or a
   validator goes stays the same. The layout is recorded in the project's
   `.gortisan.json` (`"layout": 1`); it changes only with a new major version.
4. **Your code is yours.** gortisan never rewrites a file you changed. It
   creates new files, appends declarations to the files built for it
   (`handler.go`, `dto.go`, `routes.go`, `cache.go`, `fa.json`,
   `.env.example`), and adds wiring lines to `internal/app/serve.go`.
   `--force` is the only way to overwrite a file, and only on request.
5. **Generated code needs nothing of gortisan.** No import, no line in
   `go.mod`. Upgrading gortisan never changes how the service builds or runs.

A **patch** release (`v1.2.3` → `v1.2.4`) fixes bugs. A **minor** release
(`v1.2` → `v1.3`) adds commands, flags or options. A **major** release
(`v1` → `v2`) is the only one that may change the layout or remove
something, and comes with a migration guide.

## Upgrading a project

Each project runs the gortisan version it pins, through `./gortisan`
(`gortisan.bat`, `gortisan.ps1`), so everyone on a team generates the same
code. To move a project to a newer gortisan:

```bash
./gortisan upgrade            # pin the latest release
./gortisan upgrade v1.3.0     # or a given one
```

`upgrade` changes the version in the three scripts and in `.gortisan.json`,
and nothing else. Commit those four files.

## How it is checked

- `.gortisan.json` lets every gortisan know a project's layout and
  components. A gortisan that does not know the layout refuses to generate
  code instead of writing it in the wrong place.
- `internal/generator/testdata/compat/` holds a project created by the first
  release of each layout. `TestCompatibility` runs every `make:*` of the
  current gortisan on a copy of it, then `go vet ./...` and `go test ./...`.
  A change that breaks an existing project fails CI.
