// pattern: Imperative Shell
package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"polis/internal/fixture"
	"time"
)

type Verifier struct {
	GoRoot, Scratch string
	Context         context.Context
}
type Report struct {
	Passed bool   `json:"passed"`
	Output string `json:"output"`
	Digest string `json:"digest"`
	Phase  string `json:"phase"`
}

func (v Verifier) Check(content, phase string) (Report, error) {
	limit := 45 * time.Second
	if v.Context != nil {
		if e := v.Context.Err(); e != nil {
			return Report{}, e
		}
		if deadline, ok := v.Context.Deadline(); ok && time.Until(deadline) < limit {
			limit = time.Until(deadline)
		}
	}
	if phase != "single" && phase != "full" {
		return Report{}, errors.New("unknown verifier phase")
	}
	if len(content) > 4096 {
		return Report{}, errors.New("candidate too large")
	}
	digest := sha256.Sum256([]byte(content))
	if validationErr := fixture.ValidateCandidate(content); validationErr != nil {
		return Report{Passed: false, Output: "candidate rejected: " + validationErr.Error(), Digest: hex.EncodeToString(digest[:]), Phase: phase}, nil
	}
	dir, e := os.MkdirTemp(v.Scratch, "verify-")
	if e != nil {
		return Report{}, e
	}
	defer os.RemoveAll(dir)
	for name, data := range map[string]string{"go.mod": "module fixture\n\ngo 1.26\n", "formatter.go": content, "frozen_test.go": fixture.Tests(phase)} {
		if e = os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); e != nil {
			return Report{}, e
		}
	}
	args := []string{"bwrap", "--unshare-all", "--die-with-parent", "--new-session", "--ro-bind", "/usr", "/usr", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib64", "/lib64", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--ro-bind", v.GoRoot, "/go", "--ro-bind", dir, "/candidate", "--chdir", "/candidate", "--clearenv", "--setenv", "PATH", "/go/bin:/usr/bin", "--setenv", "HOME", "/tmp", "--setenv", "GOCACHE", "/tmp/cache", "--setenv", "GOTOOLCHAIN", "local", "--setenv", "GOPROXY", "off", "--setenv", "CGO_ENABLED", "0", "/go/bin/go", "test", "-json", "-vet=off", "-count=1", "-timeout=5s", "."}
	out, runErr := Run(args, []string{"PATH=/usr/bin:/bin"}, limit)
	r := Report{Passed: runErr == nil && fixture.TestsActuallyPassed(out, phase), Output: string(out), Digest: hex.EncodeToString(digest[:]), Phase: phase}
	// Compiler/test failures are evidence of failed acceptance, not success.
	if len(out) == 0 && runErr != nil {
		return r, runErr
	}
	return r, nil
}
