package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/recovery"
)

func main() {
	directory := flag.String("directory", "", "controlled Unix socket directory")
	port := flag.Int("port", 55432, "PostgreSQL port")
	output := flag.String("output", "", "validation evidence output")
	flag.Parse()
	if *output == "" {
		fail("output is required")
	}
	report, err := recovery.ValidatePostgresUnixSocketEndpoint(*directory, *port)
	if err != nil {
		fail(err.Error())
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, append(raw, '\n'), 0o600); err != nil {
		fail(err.Error())
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
