// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"polis/db"
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
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: polis migrate | create COMPANY MISSION | start COMPANY MISSION | status COMPANY MISSION")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("POLIS_DSN")
	if os.Args[1] == "migrate" {
		return db.Migrate(ctx, dsn)
	}
	if len(os.Args) != 4 {
		return fmt.Errorf("company and mission required")
	}
	company, mission := os.Args[2], os.Args[3]
	if os.Args[1] == "status" {
		s, e := kernel.ReadSnapshot(ctx, dsn, company, mission)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(s)
	}
	root := os.Getenv("POLIS_BLOB_ROOT")
	if root == "" {
		return fmt.Errorf("POLIS_BLOB_ROOT required")
	}
	k, e := kernel.Open(ctx, dsn, root)
	if e != nil {
		return e
	}
	defer k.Close()
	scope := k.LocalScope(company)
	switch os.Args[1] {
	case "create":
		scope, e = k.TXCreateCompany(ctx, company)
		if e == nil {
			e = k.TXCreateMission(ctx, scope, mission)
		}
		if e != nil {
			return e
		}
	case "start":
		r, e := k.TXStartMission(ctx, scope, mission, "start-"+mission)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(r)
	default:
		return fmt.Errorf("unknown command")
	}
	return nil
}
