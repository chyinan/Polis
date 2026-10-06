// pattern: Functional Core
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"polis/internal/core"
)

const MaxDependencyChangePackages = 64

type DependencyChangeProposal struct {
	MissionID      string            `json:"missionId"`
	BaseRevisionID string            `json:"baseRevisionId"`
	RequestID      string            `json:"requestId"`
	RegistryHosts  []string          `json:"registryHosts"`
	Dependencies   map[string]string `json:"dependencies"`
	Rationale      string            `json:"rationale"`
}

var dependencyNamePattern = regexp.MustCompile(`^(?:@[A-Za-z0-9][A-Za-z0-9._-]*/)?[A-Za-z0-9][A-Za-z0-9._-]*$`)
var dependencyVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ValidateDependencyChangeProposal validates the owner-selected autonomous
// envelope. It describes a proposed package edit only; it never runs npm or
// produces a lockfile/environment revision.
func ValidateDependencyChangeProposal(proposal DependencyChangeProposal) error {
	if !core.ValidID(proposal.MissionID) || !core.ValidID(proposal.BaseRevisionID) || !core.ValidID(proposal.RequestID) || len(proposal.Dependencies) == 0 || len(proposal.Dependencies) > MaxDependencyChangePackages {
		return core.Malformed
	}
	if _, err := normalizeRegistryHosts(proposal.RegistryHosts); err != nil {
		return core.Malformed
	}
	if !utf8.ValidString(proposal.Rationale) || strings.TrimSpace(proposal.Rationale) == "" || len(proposal.Rationale) > 4096 {
		return core.Malformed
	}
	for name, version := range proposal.Dependencies {
		if len(name) > 128 || !dependencyNamePattern.MatchString(name) || len(version) > 128 || !validDependencyVersion(version) {
			return core.Malformed
		}
	}
	return nil
}

func validDependencyVersion(version string) bool {
	matches := dependencyVersionPattern.FindStringSubmatch(version)
	if matches == nil {
		return false
	}
	for _, identifier := range strings.Split(matches[4], ".") {
		if identifier == "" {
			continue
		}
		numeric := true
		for _, character := range identifier {
			if character < '0' || character > '9' {
				numeric = false
				break
			}
		}
		if numeric && len(identifier) > 1 && strings.HasPrefix(identifier, "0") {
			return false
		}
	}
	return true
}

func CanonicalizeDependencyChangeProposal(proposal DependencyChangeProposal) (DependencyChangeProposal, []byte, string, error) {
	if err := ValidateDependencyChangeProposal(proposal); err != nil {
		return DependencyChangeProposal{}, nil, "", err
	}
	hosts, err := normalizeRegistryHosts(proposal.RegistryHosts)
	if err != nil {
		return DependencyChangeProposal{}, nil, "", core.Malformed
	}
	proposal.RegistryHosts = hosts
	proposal.Rationale = strings.TrimSpace(proposal.Rationale)
	raw, err := json.Marshal(proposal)
	if err != nil {
		return DependencyChangeProposal{}, nil, "", err
	}
	digest := sha256.Sum256(raw)
	return proposal, raw, hex.EncodeToString(digest[:]), nil
}
