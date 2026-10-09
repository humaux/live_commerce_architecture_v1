// Package main owns the zero-dependency API contract drift gate.
// It never executes API handlers or changes application contracts.
// Purpose: provide the real CLI entrypoint for the first fixture red run.
// Depends on: Go stdlib only; no runtime services.
// Used by: CI check-gates.sh and contractdrift tests.
package main

import (
	"fmt"
	"io"
	"os"
)

func run(args []string, out, errOut io.Writer) int {
	fmt.Fprintln(out, "SUMMARY no analysis implemented")
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
