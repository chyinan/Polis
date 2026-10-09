// pattern: Imperative Shell
package main

import (
	"os"
	"polis/internal/gotestsupervisor"
)

func main() { os.Exit(gotestsupervisor.Main(os.Args[1:], os.Stdout, os.Stderr)) }
