// pattern: Functional Core
package environment

import (
	"errors"
	"testing"

	"polis/internal/core"
)

func TestValidateDependencyChangeProposalKeepsTheApprovedEnvelope(t *testing.T) {
	valid := DependencyChangeProposal{
		MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "dependency-change-1",
		RegistryHosts: []string{"registry.npmjs.org"},
		Dependencies:  map[string]string{"left-pad": "1.3.0"},
		Rationale:     "add the approved bounded dependency",
	}
	if err := ValidateDependencyChangeProposal(valid); err != nil {
		t.Fatalf("valid dependency proposal rejected: %v", err)
	}
	valid.Dependencies["left-pad"] = "1.3.0+build.7"
	if err := ValidateDependencyChangeProposal(valid); err != nil {
		t.Fatalf("valid build-metadata version rejected: %v", err)
	}
	for name, proposal := range map[string]DependencyChangeProposal{
		"missing registry":        {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", Dependencies: map[string]string{"left-pad": "1.3.0"}, Rationale: "reason"},
		"wildcard version":        {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"left-pad": "*"}, Rationale: "reason"},
		"leading zero version":    {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"left-pad": "01.3.0"}, Rationale: "reason"},
		"leading zero prerelease": {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"left-pad": "1.3.0-rc.01"}, Rationale: "reason"},
		"too many packages":       {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: make(map[string]string, MaxDependencyChangePackages+1), Rationale: "reason"},
		"missing rationale":       {MissionID: "mission-1", BaseRevisionID: "revision-1", RequestID: "request-1", RegistryHosts: []string{"registry.npmjs.org"}, Dependencies: map[string]string{"left-pad": "1.3.0"}},
	} {
		if name == "too many packages" {
			for index := 0; index <= MaxDependencyChangePackages; index++ {
				proposal.Dependencies["package-"+string(rune('a'+index%26))+string(rune('0'+index/26))] = "1.0.0"
			}
		}
		t.Run(name, func(t *testing.T) {
			if err := ValidateDependencyChangeProposal(proposal); !errors.Is(err, core.Malformed) {
				t.Fatalf("proposal validation error=%v, want malformed", err)
			}
		})
	}
}
