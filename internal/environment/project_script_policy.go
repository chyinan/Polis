// pattern: Functional Core
package environment

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	MaxNodeProjectScriptPathBytes = 260
	MaxNodeProjectScriptArgs      = 32
	MaxNodeProjectScriptArgBytes  = 1024
	MaxNodeProjectScriptArgvBytes = 8192
)

var errNodeProjectScript = errors.New("invalid Node project script request")

func NormalizeNodeProjectScriptPath(value string) (string, error) {
	if value == "" || len(value) > MaxNodeProjectScriptPathBytes || !utf8.ValidString(value) || strings.ContainsAny(value, "\\:\x00") || path.IsAbs(value) || path.Clean(value) != value || value == "." {
		return "", errNodeProjectScript
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errNodeProjectScript
		}
	}
	switch strings.ToLower(filepath.Ext(value)) {
	case ".js", ".cjs", ".mjs":
		return value, nil
	default:
		return "", errNodeProjectScript
	}
}

func ValidateNodeProjectScriptArgs(args []string) error {
	if len(args) > MaxNodeProjectScriptArgs {
		return errNodeProjectScript
	}
	total := 0
	for _, argument := range args {
		if len(argument) > MaxNodeProjectScriptArgBytes || !utf8.ValidString(argument) || strings.ContainsRune(argument, '\x00') {
			return errNodeProjectScript
		}
		total += len(argument) + 1
		if total > MaxNodeProjectScriptArgvBytes {
			return errNodeProjectScript
		}
	}
	return nil
}
