// pattern: Functional Core
package provider

import (
	"context"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

const ProviderAuthIdentitySnapshotSchemaV1 = "provider-auth-identity@1"

var providerAuthIdentityLabel = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

type ProviderAuthIdentitySnapshot struct {
	SchemaVersion string `json:"schema_version"`
	SourceClass   string `json:"source_class"`
	Status        string `json:"status"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
}

func (s ProviderAuthIdentitySnapshot) Validate() error {
	if s.SchemaVersion != ProviderAuthIdentitySnapshotSchemaV1 || !providerAuthIdentityLabel.MatchString(s.SourceClass) {
		return errors.New("provider auth identity snapshot metadata is invalid")
	}
	switch s.Status {
	case "available":
		raw, err := hex.DecodeString(s.Fingerprint)
		if err != nil || len(raw) != 32 || s.Fingerprint != strings.ToLower(s.Fingerprint) || s.ReasonCode != "" {
			return errors.New("provider auth identity fingerprint is invalid")
		}
	case "unavailable", "unsupported":
		if s.Fingerprint != "" || !providerAuthIdentityLabel.MatchString(s.ReasonCode) {
			return errors.New("provider auth identity absence reason is invalid")
		}
	default:
		return errors.New("provider auth identity status is invalid")
	}
	return nil
}

// ProviderAuthIdentitySnapshotSource is optional so deterministic and other
// provider runtimes can state that they do not expose an account principal.
type ProviderAuthIdentitySnapshotSource interface {
	ProviderAuthIdentitySnapshot(context.Context) (ProviderAuthIdentitySnapshot, error)
}

func CaptureProviderAuthIdentitySnapshot(ctx context.Context, runtime Runtime) (ProviderAuthIdentitySnapshot, error) {
	if ctx == nil || runtime == nil {
		return ProviderAuthIdentitySnapshot{}, errors.New("provider auth identity source is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return ProviderAuthIdentitySnapshot{}, err
	}
	if source, ok := runtime.(ProviderAuthIdentitySnapshotSource); ok {
		snapshot, err := source.ProviderAuthIdentitySnapshot(ctx)
		if err != nil {
			return ProviderAuthIdentitySnapshot{}, err
		}
		if err = snapshot.Validate(); err != nil {
			return ProviderAuthIdentitySnapshot{}, err
		}
		return snapshot, nil
	}
	return ProviderAuthIdentitySnapshot{
		SchemaVersion: ProviderAuthIdentitySnapshotSchemaV1,
		SourceClass:   "unsupported_runtime",
		Status:        "unsupported",
		ReasonCode:    "runtime_identity_capability_missing",
	}, nil
}
