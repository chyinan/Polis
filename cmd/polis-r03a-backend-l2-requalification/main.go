// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-current-binary-backend-l2-v2", "new Backend L2 evidence path")
	oldEvidence := flag.String("old-evidence", "evidence/development/r0.3a-current-binary-l2", "immutable prior Backend L2 evidence")
	l1Evidence := flag.String("l1-evidence", "evidence/development/r0.3a-current-binary-l1", "qualified current-binary L1 evidence")
	acceptance := flag.String("acceptance", "evidence/development/r0.3a-pagination-acceptance-remediation/qualification.json", "qualified business acceptance evidence")
	binary := flag.String("binary", `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe`, "controlled Codex binary")
	helper := flag.String("helper", `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe`, "controlled code-mode host")
	auth := flag.String("auth", `C:\Users\chyinan\.codex\auth.json`, "controlled diagnostic auth")
	config := flag.String("config", `D:\Programs\Polis\.runtime\linux\r03a-t14c\home\config.toml`, "qualified config")
	runtimeManifest := flag.String("runtime-manifest", `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\runtime-manifest.json`, "controlled runtime manifest")
	flag.Parse()
	record, err := probe.RecordR03ABackendL2Requalification(probe.BackendL2RequalificationConfig{Evidence: *evidence, OldEvidence: *oldEvidence, L1Evidence: *l1Evidence, AcceptanceQualification: *acceptance, Binary: *binary, CodeModeHost: *helper, AuthFile: *auth, SelectedConfig: *config, RuntimeManifest: *runtimeManifest})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := probe.VerifyR03ABackendL2Requalification(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Backend current-binary L2 offline qualification passed: %s execution=%v\n", *evidence, record["execution_fingerprint"])
}
