// pattern: Functional Core
package provider

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
)

const ProviderAccountIdentitySnapshotSchemaV1 = "provider-account-identity@1"

type ProviderAccountIdentitySnapshot struct {
	SchemaVersion string `json:"schema_version"`
	ProviderClass string `json:"provider_class"`
	Status        string `json:"status"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
}

func (s ProviderAccountIdentitySnapshot) Validate() error {
	if s.SchemaVersion != ProviderAccountIdentitySnapshotSchemaV1 || !providerAuthIdentityLabel.MatchString(s.ProviderClass) {
		return errors.New("provider account identity snapshot metadata is invalid")
	}
	switch s.Status {
	case "available":
		raw, err := hex.DecodeString(s.Fingerprint)
		if err != nil || len(raw) != 32 || s.Fingerprint != strings.ToLower(s.Fingerprint) || s.ReasonCode != "" {
			return errors.New("provider account identity fingerprint is invalid")
		}
	case "unavailable", "unsupported":
		if s.Fingerprint != "" || !providerAuthIdentityLabel.MatchString(s.ReasonCode) {
			return errors.New("provider account identity absence reason is invalid")
		}
	default:
		return errors.New("provider account identity status is invalid")
	}
	return nil
}

// ProviderAccountIdentitySnapshotSource is optional because many provider
// auth modes do not expose a stable account locator.
type ProviderAccountIdentitySnapshotSource interface {
	ProviderAccountIdentitySnapshot(context.Context) (ProviderAccountIdentitySnapshot, error)
}

func CaptureProviderAccountIdentitySnapshot(ctx context.Context, runtime Runtime) (ProviderAccountIdentitySnapshot, error) {
	if ctx == nil || runtime == nil {
		return ProviderAccountIdentitySnapshot{}, errors.New("provider account identity source is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return ProviderAccountIdentitySnapshot{}, err
	}
	if source, ok := runtime.(ProviderAccountIdentitySnapshotSource); ok {
		snapshot, err := source.ProviderAccountIdentitySnapshot(ctx)
		if err != nil {
			return ProviderAccountIdentitySnapshot{}, err
		}
		if err = snapshot.Validate(); err != nil {
			return ProviderAccountIdentitySnapshot{}, err
		}
		return snapshot, nil
	}
	return ProviderAccountIdentitySnapshot{
		SchemaVersion: ProviderAccountIdentitySnapshotSchemaV1,
		ProviderClass: "unsupported_runtime",
		Status:        "unsupported",
		ReasonCode:    "runtime_account_identity_missing",
	}, nil
}
