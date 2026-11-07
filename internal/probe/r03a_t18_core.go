// pattern: Functional Core
package probe

import "strings"

type T18Assessment struct {
	CandidateConstructible      bool              `json:"candidate_constructible"`
	SelectedConfigKeys          []string          `json:"selected_config_keys"`
	OmittedConfigKeys           map[string]string `json:"omitted_config_keys"`
	EffectiveConfigDigestBefore string            `json:"effective_config_digest_before"`
	EffectiveConfigDigestAfter  string            `json:"effective_config_digest_after"`
	TransportConfigDigestBefore string            `json:"transport_config_digest_before"`
	TransportConfigDigestAfter  string            `json:"transport_config_digest_after"`
	UnchangedFactors            []string          `json:"unchanged_factors"`
	Reason                      string            `json:"reason"`
}

func AssessT18SelectedConfig(fields map[string]T17FieldComparison, effectiveBefore, transportBefore string) T18Assessment {
	assessment := T18Assessment{
		SelectedConfigKeys:          []string{},
		OmittedConfigKeys:           map[string]string{},
		EffectiveConfigDigestBefore: effectiveBefore,
		EffectiveConfigDigestAfter:  effectiveBefore,
		TransportConfigDigestBefore: transportBefore,
		TransportConfigDigestAfter:  transportBefore,
		UnchangedFactors: []string{
			"codex_version", "binary_sha256", "runtime", "invocation", "model", "effort", "sandbox", "network_namespace_policy", "proxy_policy", "dynamic_tools", "plugins", "mcp", "auth_source_class", "auth_identity_fingerprint", "auth_credential_revision_fingerprint", "prompt", "deadlines", "stop_semantics",
		},
		Reason: "no single safe selected config key can be identified from T17 without changing a fixed factor or leaving default resolution unproven",
	}
	for name, field := range fields {
		if field.Comparison == T17ConfirmedSame || field.Comparison == T17Absent {
			continue
		}
		assessment.OmittedConfigKeys[name] = t18OmissionReason(name)
	}
	return assessment
}

func t18OmissionReason(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "reasoning"):
		return "prohibited: T18 keeps effort=medium"
	case strings.Contains(lower, "sandbox"):
		return "prohibited: T18 keeps read-only sandbox"
	case strings.Contains(lower, "network"):
		return "fixed by T16 network qualification; changing it is not config-loading-only"
	case strings.Contains(lower, "websocket") || strings.Contains(lower, "transport"):
		return "default resolution is unproven across Windows and WSL/Linux"
	case strings.Contains(lower, "plugin") || strings.Contains(lower, "mcp") || strings.Contains(lower, "feature") || strings.Contains(lower, "tool"):
		return "prohibited: plugins/MCP/tools remain disabled"
	case strings.Contains(lower, "auth") || strings.Contains(lower, "credential"):
		return "credential semantics are controlled separately from config"
	case strings.Contains(lower, "base_url") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "proxy") || strings.Contains(lower, "shell_environment"):
		return "sensitive or provider endpoint semantics; not a safe selected key"
	case strings.Contains(lower, "provider"):
		return "provider selection is a provider-visible factor, not isolated config loading"
	default:
		return "no direct evidence that this key changes only config-loading semantics"
	}
}
