// pattern: Functional Core
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	T17ConfirmedSame      = "confirmed_same"
	T17ConfirmedDifferent = "confirmed_different"
	T17DefaultInA         = "default_in_A"
	T17DefaultInB         = "default_in_B"
	T17Absent             = "absent"
	T17Unknown            = "unknown"
	T17SensitiveRedacted  = "sensitive_redacted"

	T17CredentialClass     = "credentials"
	T17ConfigurationClass  = "configuration"
	T17CacheClass          = "cache"
	T17EphemeralStateClass = "ephemeral_session_state"
	T17OtherStateClass     = "other_state"
)

type T17ParsedConfig struct {
	Exists          bool              `json:"exists"`
	Fields          map[string]string `json:"fields"`
	SensitiveFields map[string]bool   `json:"-"`
}

type T17FieldComparison struct {
	Name            string `json:"name"`
	AClassification string `json:"a_classification"`
	BClassification string `json:"b_classification"`
	AValue          string `json:"a_value,omitempty"`
	BValue          string `json:"b_value,omitempty"`
	Comparison      string `json:"comparison"`
}

func ParseT17Config(raw []byte, exists bool) (T17ParsedConfig, error) {
	parsed := T17ParsedConfig{Exists: exists, Fields: map[string]string{}}
	parsed.SensitiveFields = map[string]bool{}
	if !exists {
		return parsed, nil
	}
	section := ""
	for _, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		separator := strings.IndexByte(line, '=')
		if separator <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:separator])
		value := strings.TrimSpace(line[separator+1:])
		if key == "" {
			continue
		}
		fullName := key
		if section != "" {
			fullName = section + "." + key
		}
		parsed.SensitiveFields[fullName] = isT17SensitiveField(fullName)
		parsed.Fields[fullName] = normalizeT17Value(fullName, value)
	}
	return parsed, nil
}

func CompareT17ConfigFields(a, b T17ParsedConfig, names []string) map[string]T17FieldComparison {
	result := make(map[string]T17FieldComparison, len(names))
	for _, name := range names {
		aValue, aPresent := a.Fields[name]
		bValue, bPresent := b.Fields[name]
		field := T17FieldComparison{Name: name, AClassification: T17Absent, BClassification: T17Absent, Comparison: T17Unknown}
		aSensitive := a.SensitiveFields[name]
		bSensitive := b.SensitiveFields[name]
		if aPresent {
			field.AClassification, field.AValue = T17ConfirmedDifferent, aValue
			if aSensitive {
				field.AClassification = T17SensitiveRedacted
			}
		}
		if bPresent {
			field.BClassification, field.BValue = T17ConfirmedDifferent, bValue
			if bSensitive {
				field.BClassification = T17SensitiveRedacted
			}
		}
		if !aPresent && !bPresent {
			field.AClassification, field.BClassification = T17Absent, T17Absent
			field.Comparison = T17Absent
		} else if aPresent && bPresent && (aSensitive || bSensitive) {
			field.AClassification, field.BClassification, field.Comparison = T17SensitiveRedacted, T17SensitiveRedacted, T17SensitiveRedacted
		} else if aPresent && bPresent && aValue == bValue {
			field.AClassification, field.BClassification, field.Comparison = T17ConfirmedSame, T17ConfirmedSame, T17ConfirmedSame
		} else if aPresent && bPresent {
			field.AClassification, field.BClassification, field.Comparison = T17ConfirmedDifferent, T17ConfirmedDifferent, T17ConfirmedDifferent
		} else if !aPresent && bValue == T17DefaultInB {
			field.AClassification, field.BClassification, field.Comparison = T17Unknown, T17DefaultInB, T17Unknown
		} else if !bPresent && aValue == T17DefaultInA {
			field.AClassification, field.BClassification, field.Comparison = T17DefaultInA, T17Unknown, T17Unknown
		}
		result[name] = field
	}
	return result
}

func ClassifyT17File(name string) string {
	base := strings.ToLower(name)
	switch {
	case base == "auth.json" || strings.Contains(base, "credential"):
		return T17CredentialClass
	case base == "config.toml" || strings.Contains(base, "hooks.json") || strings.Contains(base, "rules"):
		return T17ConfigurationClass
	case strings.Contains(base, "cache") || strings.Contains(base, "model"):
		return T17CacheClass
	case strings.Contains(base, "history") || strings.Contains(base, ".sqlite") || strings.Contains(base, "queue") || strings.Contains(base, "state") || strings.Contains(base, "goal") || strings.Contains(base, "log") || strings.Contains(base, "memory") || strings.Contains(base, "session"):
		return T17EphemeralStateClass
	default:
		return T17OtherStateClass
	}
}

func T17ConfigDigest(config T17ParsedConfig) string {
	material, _ := json.Marshal(config.Fields)
	digest := sha256.Sum256(material)
	return hex.EncodeToString(digest[:])
}

func T17TransportConfigDigest(config T17ParsedConfig, fields []string) string {
	selected := make(map[string]string, len(fields))
	for _, field := range fields {
		if value, ok := config.Fields[field]; ok {
			selected[field] = value
		} else {
			selected[field] = T17Unknown
		}
	}
	material, _ := json.Marshal(selected)
	digest := sha256.Sum256(material)
	return hex.EncodeToString(digest[:])
}

func normalizeT17Value(field, value string) string {
	if isT17SensitiveField(field) {
		return T17SensitiveRedacted
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func isT17SensitiveField(field string) bool {
	lower := strings.ToLower(field)
	for _, marker := range []string{"token", "secret", "password", "cookie", "id_token", "refresh_token", "auth_token", "base_url", "endpoint", "proxy", "command", "args", "source", "trusted_services"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
