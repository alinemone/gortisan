// Command layout1 is the Layout1 service. Its commands are in cmd/, one
// file each; `go run . --help` lists them.
package main

import "example.com/compat/cmd"

// General API information for Swagger. swag reads these comments
// (`./gortisan swag`) and writes docs/; each endpoint is documented on its
// handler.
//
//	@title		Layout1 API
//	@version	1.0
//	@BasePath	/api
func main() {
	cmd.Execute()
}
