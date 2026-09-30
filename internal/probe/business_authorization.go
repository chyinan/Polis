// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"polis/internal/codex"
)

// RequireBusinessAuthorization validates the immutable business allowance
// binding immediately before a worker may be created or an allowance used.
// It performs no writes and delegates all matching rules to the pure codex
// authorization core.
func RequireBusinessAuthorization(bindingPath string, context codex.BusinessExecutionContext) error {
	_, err := LoadBusinessAuthorization(bindingPath, context)
	return err
}

func LoadBusinessAuthorization(bindingPath string, context codex.BusinessExecutionContext) (codex.BusinessAuthorizationBinding, error) {
	var empty codex.BusinessAuthorizationBinding
	if bindingPath == "" {
		return empty, errors.New("business authorization blocked: authorization binding is required")
	}
	raw, err := os.ReadFile(bindingPath)
	if err != nil {
		return empty, fmt.Errorf("business authorization blocked: read binding: %w", err)
	}
	var binding codex.BusinessAuthorizationBinding
	if err := json.Unmarshal(raw, &binding); err != nil {
		return empty, fmt.Errorf("business authorization blocked: invalid binding: %w", err)
	}
	decision := codex.AuthorizeBusinessExecution(binding, context)
	if !decision.Allowed {
		return empty, fmt.Errorf("business authorization blocked: reason=%s", decision.ReasonCode)
	}
	return binding, nil
}
