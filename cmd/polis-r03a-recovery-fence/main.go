// pattern: Imperative Shell
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"polis/internal/kernel"
	"time"
)

func main() {
	dsn := flag.String("dsn", "", "restored disposable PostgreSQL DSN")
	root := flag.String("root", "", "restored external blob root")
	company := flag.String("company", "recovery-company", "company identity")
	session := flag.String("session", "recovery-backend-session", "historical worker session")
	employee := flag.String("employee", "emp-backend", "historical employee")
	incarnation := flag.String("incarnation", "source-incarnation", "historical runtime incarnation")
	epoch := flag.Int64("epoch", 9, "historical employee epoch")
	flag.Parse()
	if *dsn == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "dsn and root are required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, err := kernel.Open(ctx, *dsn, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer k.Close()
	if err := k.VerifyHistoricalWorkerFence(ctx, *company, *session, *employee, *incarnation, *epoch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(`{"historical_worker_fenced":true,"business_write_attempted":false}`)
}
