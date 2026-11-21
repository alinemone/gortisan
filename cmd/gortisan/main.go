// Command gortisan writes Go service code: a new project with `create`, and
// the pieces of a project with `make:*`. The code it writes never imports
// gortisan, and a project's go.mod does not list it; after generation every
// file belongs to the project.
//
//	go install github.com/alinemone/gortisan/cmd/gortisan@latest
//	gortisan create shop github.com/you/shop
//	gortisan make:feature user        (or: make feature user)
//
// Inside a project, the generated ./gortisan script (gortisan.bat,
// gortisan.ps1 on Windows) runs the version the project pins, through
// `go run`, so a team needs nothing installed.
//
// Each command is one file of this package; root.go lists them all.
package main

import "os"

func main() {
	os.Exit(execute(os.Args[1:]))
}
