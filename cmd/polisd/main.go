// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/kernel"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	company := flag.String("company", "", "test company")
	steps := flag.Int("steps", 8, "maximum fake boundaries; exits immediately when idle")
	flag.Parse()
	if *company == "" || *steps < 1 || *steps > 100 {
		return fmt.Errorf("company and steps (1..100) required")
	}
	root := os.Getenv("POLIS_BLOB_ROOT")
	if root == "" {
		return fmt.Errorf("POLIS_BLOB_ROOT required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, e := kernel.Open(ctx, os.Getenv("POLIS_DSN"), root)
	if e != nil {
		return e
	}
	defer k.Close()
	count := 0
	for ; count < *steps; count++ {
		worked, e := k.Step(ctx, k.LocalScope(*company))
		if e != nil {
			return e
		}
		if !worked {
			break
		}
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Mode  string `json:"mode"`
		Steps int    `json:"steps"`
	}{"fake_only", count})
}
