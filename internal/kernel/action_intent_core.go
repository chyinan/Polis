// pattern: Functional Core
package kernel

import (
	"strings"

	"polis/internal/core"
)

const GenericActionIntentDeniedReason = "generic_dispatch_permit_unavailable"

type GenericActionIntentRequest struct {
	ActionKind     string `json:"actionKind"`
	ResourceKey    string `json:"resourceKey"`
	TargetSHA256   string `json:"targetSha256"`
	InputSHA256    string `json:"inputSha256"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func normalizeGenericActionIntentRequest(input GenericActionIntentRequest) (GenericActionIntentRequest, error) {
	input.ActionKind = strings.TrimSpace(input.ActionKind)
	input.ResourceKey = strings.TrimSpace(input.ResourceKey)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ActionKind != "shared_write" && input.ActionKind != "external_write" && input.ActionKind != "notification" && input.ActionKind != "provider_request" {
		return GenericActionIntentRequest{}, core.Malformed
	}
	if !validGenericResourceKey(input.ResourceKey) || !validEnvironmentSHA256(input.TargetSHA256) || !validEnvironmentSHA256(input.InputSHA256) || !core.ValidID(input.IdempotencyKey) {
		return GenericActionIntentRequest{}, core.Malformed
	}
	return input, nil
}

func validGenericResourceKey(value string) bool {
	if value == "" || len(value) > 256 || strings.ContainsAny(value, "\r\n\t") || strings.Contains(value, "//") {
		return false
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[1], ":") {
		return false
	}
	for _, char := range parts[0] {
		if !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '_' && char != '-' && char != '.' {
			return false
		}
	}
	return true
}
