package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"polis/internal/installationauth"
)

func runOwnerBootstrap() error {
	if len(os.Args) != 2 {
		return errors.New("usage: polis owner-bootstrap")
	}
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("owner bootstrap code is printed only to an interactive terminal")
	}
	dsn := os.Getenv("POLIS_DSN")
	if dsn == "" {
		return errors.New("POLIS_DSN is required for local owner bootstrap")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := installationauth.OpenStore(ctx, dsn)
	if err != nil {
		return errors.New("failed to open the local installation owner store")
	}
	defer store.Close()
	code, expiresAt, err := store.IssueBootstrap(ctx)
	if err != nil {
		return fmt.Errorf("failed to issue owner bootstrap code: %w", err)
	}
	if _, err = fmt.Fprintf(os.Stdout, "One-time owner bootstrap code (expires %s):\n%s\nEnter it only in the local Polis Workbench.\n", expiresAt.Format(time.RFC3339), code); err != nil {
		return errors.New("failed to write owner bootstrap code to the terminal")
	}
	return nil
}
