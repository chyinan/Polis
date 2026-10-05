package spec

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

//go:embed design-v0.4.5/product/TEAM_COVERAGE.json
var fixedTeamCoverage []byte

// FixedTeamCoverageSHA256 identifies the exact reviewed team coverage draft.
func FixedTeamCoverageSHA256() string {
	digest := sha256.Sum256(fixedTeamCoverage)
	return hex.EncodeToString(digest[:])
}
