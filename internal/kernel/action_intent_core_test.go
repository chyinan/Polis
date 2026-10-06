// pattern: Functional Core
package kernel

import (
	"errors"
	"polis/internal/core"
	"strings"
	"testing"
)

func TestNormalizeGenericActionIntentUsesCanonicalBoundedResourceIdentity(t *testing.T) {
	valid := GenericActionIntentRequest{ActionKind: "shared_write", ResourceKey: "github:repo-123/ref-main", TargetSHA256: strings.Repeat("a", 64), InputSHA256: strings.Repeat("b", 64), IdempotencyKey: "action-1"}
	if got, err := normalizeGenericActionIntentRequest(valid); err != nil || got.ResourceKey != valid.ResourceKey {
		t.Fatalf("valid action intent=%+v err=%v", got, err)
	}
	for name, mutate := range map[string]func(*GenericActionIntentRequest){
		"unknown action":    func(input *GenericActionIntentRequest) { input.ActionKind = "delete_everything" },
		"alias resource":    func(input *GenericActionIntentRequest) { input.ResourceKey = "github://repo-123" },
		"bad target digest": func(input *GenericActionIntentRequest) { input.TargetSHA256 = "bad" },
		"bad idempotency":   func(input *GenericActionIntentRequest) { input.IdempotencyKey = "" },
	} {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if _, err := normalizeGenericActionIntentRequest(input); !errors.Is(err, core.Malformed) {
				t.Fatalf("normalize error=%v, want %v", err, core.Malformed)
			}
		})
	}
}
