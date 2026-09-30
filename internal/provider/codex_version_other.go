//go:build !windows

// pattern: Imperative Shell
package provider

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func readCodexBinaryVersion(ctx context.Context, binary string) (string, error) {
	command := exec.CommandContext(ctx, binary, "--version")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("Codex version query failed: %w", err)
	}
	version := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if version == "" {
		return "", errors.New("Codex version query returned an empty version")
	}
	return version, nil
}
