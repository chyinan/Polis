// pattern: Functional Core
package probe

import (
	"errors"
	"fmt"
	"reflect"

	"polis/internal/codex"
)

type T14CanaryFactors struct {
	Combination                codex.ExecutionCombination
	AuthManifest               codex.AuthFingerprintManifest
	ProxyEnvironment           map[string]string
	CWD                        string
	Home                       string
	Invocation                 string
	Sandbox                    string
	WebSocketPolicy            string
	DynamicToolCount           int
	ToolSchemaBytes            int
	DeveloperInstructionDigest string
	PromptDigest               string
	Transport                  codex.TransportPolicyManifest
}

type T14FactorProof struct {
	Passed                 bool                         `json:"passed"`
	ConfirmedDivergence    []string                     `json:"confirmed_divergence"`
	NonTransportDivergence []string                     `json:"non_transport_divergence,omitempty"`
	AuthComparison         codex.AuthManifestComparison `json:"auth_comparison"`
	OldV3Fingerprint       string                       `json:"old_v3_fingerprint"`
	NewV3Fingerprint       string                       `json:"new_v3_fingerprint"`
}

var t14TransportDerivedFields = []string{"combination.capability_digest"}

func CompareT14Factors(old, next T14CanaryFactors) (T14FactorProof, error) {
	proof := T14FactorProof{
		AuthComparison:   codex.CompareAuthManifests(old.AuthManifest, next.AuthManifest),
		OldV3Fingerprint: old.Combination.CanonicalManifestV3Digest(old.AuthManifest, old.Transport),
		NewV3Fingerprint: next.Combination.CanonicalManifestV3Digest(next.AuthManifest, next.Transport),
	}
	if err := old.Combination.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T13 combination: %w", err)
	}
	if err := next.Combination.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T14 combination: %w", err)
	}
	if err := old.AuthManifest.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T13 auth manifest: %w", err)
	}
	if err := next.AuthManifest.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T14 auth manifest: %w", err)
	}
	if err := old.Transport.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T13 transport policy: %w", err)
	}
	if err := next.Transport.Validate(); err != nil {
		return proof, fmt.Errorf("preflight_failed: invalid T14 transport policy: %w", err)
	}

	oldFields := combinationFields(old.Combination)
	nextFields := combinationFields(next.Combination)
	for field, oldValue := range oldFields {
		if oldValue == nextFields[field] {
			continue
		}
		if field == "combination.capability_digest" {
			proof.ConfirmedDivergence = append(proof.ConfirmedDivergence, field)
			continue
		}
		proof.NonTransportDivergence = append(proof.NonTransportDivergence, field)
	}
	if old.Transport.ProviderTransportPolicy != next.Transport.ProviderTransportPolicy {
		proof.ConfirmedDivergence = append(proof.ConfirmedDivergence, "transport.provider_transport_policy")
	}
	if old.Transport.WebSocketPolicy != next.Transport.WebSocketPolicy {
		proof.ConfirmedDivergence = append(proof.ConfirmedDivergence, "transport.websocket_policy")
	}
	if proof.AuthComparison.SourceClass != codex.AuthComparisonSame || proof.AuthComparison.Identity != codex.AuthComparisonSame || proof.AuthComparison.CredentialRevision != codex.AuthComparisonSame {
		proof.NonTransportDivergence = append(proof.NonTransportDivergence, "auth_manifest")
	}
	if !reflect.DeepEqual(old.ProxyEnvironment, next.ProxyEnvironment) {
		proof.NonTransportDivergence = append(proof.NonTransportDivergence, "proxy_environment")
	}
	for field, oldValue := range map[string]string{"cwd": old.CWD, "home": old.Home, "invocation": old.Invocation, "sandbox": old.Sandbox, "developer_instruction_digest": old.DeveloperInstructionDigest, "prompt_digest": old.PromptDigest} {
		if oldValue != factorStringT14(next, field) {
			proof.NonTransportDivergence = append(proof.NonTransportDivergence, field)
		}
	}
	if old.DynamicToolCount != next.DynamicToolCount {
		proof.NonTransportDivergence = append(proof.NonTransportDivergence, "dynamic_tool_count")
	}
	if old.ToolSchemaBytes != next.ToolSchemaBytes {
		proof.NonTransportDivergence = append(proof.NonTransportDivergence, "tool_schema_bytes")
	}
	if len(proof.NonTransportDivergence) != 0 {
		return proof, errors.New("preflight_failed: confirmed divergence outside WebSocket transport policy: " + fmt.Sprint(proof.NonTransportDivergence))
	}
	proof.Passed = true
	return proof, nil
}

func factorStringT14(f T14CanaryFactors, field string) string {
	switch field {
	case "cwd":
		return f.CWD
	case "home":
		return f.Home
	case "invocation":
		return f.Invocation
	case "sandbox":
		return f.Sandbox
	case "websocket_policy":
		return f.WebSocketPolicy
	case "developer_instruction_digest":
		return f.DeveloperInstructionDigest
	case "prompt_digest":
		return f.PromptDigest
	default:
		return ""
	}
}
