// pattern: Functional Core
package fixture

// Baseline and verifier revisions are trusted, frozen inputs outside the worker view.
const Revision = "signed-zero@1"
const Source = `package formatter

import (
 "math"
 "strconv"
)

func Render(v float64) string {
 if v == 0 {
  if math.Signbit(v) { return "-0" }
  return "0"
 }
 return strconv.FormatFloat(v, 'f', -1, 64)
}
`
const SingleInstructions = "Modify formatter.go so Render adds + for strictly positive numbers. Preserve existing behavior for negative numbers and signed zero, including fractional values. Use workspace_read, workspace_replace and workspace_check. Persist a work_checkpoint with evidence, facts, decisions and rejected alternatives, then artifact_submit. Only the supplied tools are permitted."
const PartialInstructions = "Implement only the first milestone: Render must add + for strictly positive numbers and preserve existing negative/zero behavior. Inspect the baseline carefully and use workspace_check to obtain executable evidence. Before completing the overall task, persist a work_checkpoint documenting the discovered compatibility constraint, decisions, a rejected approach and evidence IDs. Do NOT submit an artifact yet; a later employee session will implement optional units."
const FinalInstructions = "Continue using the neutral handover bundle and current workspace. Retain the already implemented Render behavior. Add RenderWithUnit(v float64, unit string) string, returning Render(v) plus one space and unit when unit is nonempty; empty unit returns Render(v). Preserve the compatibility constraints discovered earlier. Use workspace_check, then persist a work_checkpoint and artifact_submit. Do not change the existing Render signature."

func Tests(phase string) string {
	common := `package formatter
import ("testing"; "math")
func TestFrozenCompatibility(t *testing.T) {
 cases:=[]struct{v float64; want string}{{math.Copysign(0,-1),"-0"},{0,"0"},{-2.5,"-2.5"},{3.25,"+3.25"},{1e-6,"+0.000001"}}
 for _,c:=range cases { if got:=Render(c.v);got!=c.want {t.Errorf("Render(%v): got %q, want %q",c.v,got,c.want)} }
}
`
	if phase == "full" {
		common += `
func TestFrozenNeighborChange(t *testing.T) {
 cases:=[]struct{v float64;unit,want string}{{math.Copysign(0,-1),"kg","-0 kg"},{0,"kg","0 kg"},{4.5,"m","+4.5 m"},{-7,"m","-7 m"},{math.Copysign(0,-1),"","-0"},{2,"","+2"}}
 for _,c:=range cases{if got:=RenderWithUnit(c.v,c.unit);got!=c.want{t.Errorf("unit result: got %q, want %q",got,c.want)}}
}
`
	}
	return common
}
