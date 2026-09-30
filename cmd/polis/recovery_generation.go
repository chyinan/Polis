// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"polis/internal/recovery"
)

func verifyRestoredRecoveryGeneration() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: polis recovery-generation-verify PACKAGE_DIRECTORY BLOB_ROOT")
	}
	databaseDSN := strings.TrimSpace(os.Getenv("POLIS_GENERATION_DSN"))
	if databaseDSN == "" {
		return errors.New("POLIS_GENERATION_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := recovery.VerifyRestoredRecoveryGeneration(ctx, os.Args[2], databaseDSN, os.Args[3])
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
