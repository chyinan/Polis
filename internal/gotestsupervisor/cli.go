// pattern: Imperative Shell
package gotestsupervisor

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

var SourceIdentity string // injected by the dedicated build script
type BinaryIdentity struct {
	Protocol        string `json:"protocol"`
	Profile         string `json:"profile"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	BinarySHA256    string `json:"binarySha256"`
	SourceIdentity  string `json:"sourceIdentity"`
	ToolchainSHA256 string `json:"toolchainSha256"`
}

func sourceIdentity(root string) (string, error) {
	var refs []string
	for _, dir := range []string{"internal", "cmd/polis-selfdev-go-job-supervisor"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return ErrInvalid
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			s, e := fileSHA(path)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			refs = append(refs, filepath.ToSlash(rel)+"\x00"+s)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		s, e := fileSHA(filepath.Join(root, name))
		if e != nil {
			return "", e
		}
		refs = append(refs, name+"\x00"+s)
	}
	sort.Strings(refs)
	return SHA256([]byte(strings.Join(refs, "\n"))), nil
}
func Main(args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("polis-selfdev-go-job-supervisor", flag.ContinueOnError)
	f.SetOutput(errout)
	info := f.Bool("protocol-info", false, "print binary identity; does not execute candidate code")
	inspect := f.Bool("inspect", false, "read-only host preflight")
	check := f.String("check-input", "", "validate a canonical private request file without executing it")
	job := f.String("job-id", "", "fixed-profile JobRun identifier; request comes from its systemd runtime directory")
	source := f.String("source-identity", "", "hash current production source without compiling or executing it")
	verify := f.String("verify-build", "", "verify an identity manifest against this binary")
	sourceRoot := f.String("source-root", "", "repository root used with --verify-build")
	cfg := DefaultConfig()
	diagnostic := func(reason string) {
		_ = json.NewEncoder(out).Encode(struct {
			Protocol              string `json:"protocol"`
			Reason                string `json:"reason"`
			ProcessStarted        bool   `json:"processStarted"`
			ExitObserved          bool   `json:"exitObserved"`
			QualificationEligible bool   `json:"qualificationEligible"`
		}{Protocol: Protocol, Reason: reason})
	}
	f.StringVar(&cfg.ToolchainRoot, "toolchain-root", cfg.ToolchainRoot, "approved hash-pinned Go toolchain root")
	if e := f.Parse(args); e != nil {
		if e == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 {
		diagnostic("invalid_arguments")
		return 2
	}
	actions := 0
	for _, b := range []bool{*info, *inspect, *check != "", *source != "", *verify != ""} {
		if b {
			actions++
		}
	}
	if actions > 1 || *job != "" && !core.ValidID(*job) {
		diagnostic("invalid_action_or_job_identity")
		fmt.Fprintln(errout, "invalid mutually exclusive action or job identifier")
		return 2
	}
	identity := func() (BinaryIdentity, error) {
		s, e := ExecutableSHA256()
		return BinaryIdentity{Protocol, gotestprofile.FixtureRevision, runtime.GOOS, runtime.GOARCH, s, SourceIdentity, gotestprofile.FixtureToolchainSHA256}, e
	}
	if *source != "" {
		s, e := sourceIdentity(*source)
		if e != nil {
			fmt.Fprintln(errout, "source identity unavailable")
			return 2
		}
		fmt.Fprintln(out, s)
		return 0
	}
	if *info {
		id, e := identity()
		if e != nil {
			return 2
		}
		_ = json.NewEncoder(out).Encode(id)
		return 0
	}
	if *verify != "" {
		i, e := os.Lstat(*verify)
		if e != nil || !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 || i.Size() > 65536 {
			return 2
		}
		raw, e := os.ReadFile(*verify)
		if e != nil {
			return 2
		}
		var expected BinaryIdentity
		if json.Unmarshal(raw, &expected) != nil {
			return 2
		}
		actual, e := identity()
		if e != nil || actual != expected || actual.OS != "linux" || actual.Arch != "amd64" || !validDigest(actual.SourceIdentity) || *sourceRoot == "" {
			return 2
		}
		current, e := sourceIdentity(*sourceRoot)
		if e != nil || current != actual.SourceIdentity {
			fmt.Fprintln(errout, "supervisor source identity mismatch; rebuild required")
			return 2
		}
		fmt.Fprintln(out, "supervisor_build=verified")
		return 0
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *inspect {
		h := Inspect(ctx, *job, cfg, "")
		_ = json.NewEncoder(out).Encode(h)
		if !h.Qualified {
			return 3
		}
		return 0
	}
	path := *check
	if path == "" {
		if *job == "" {
			f.Usage()
			return 2
		}
		path = managedRequestPath(*job)
	}
	raw, e := validatePrivateRequest(path)
	if e != nil {
		fmt.Fprintln(errout, "private bounded request unavailable")
		diagnostic("private_request_unavailable")
		return 2
	}
	r, e := ParseRequest(raw)
	if e != nil || *job != "" && r.Binding.JobRunID != *job {
		fmt.Fprintln(errout, "invalid request identity")
		diagnostic("invalid_request_identity")
		return 2
	}
	if *check != "" {
		fmt.Fprintln(out, "request=valid")
		return 0
	}
	report := Execute(ctx, r, cfg)
	_ = json.NewEncoder(out).Encode(report)
	// Keep the authoritative report available to the control plane independently
	// of journald. Candidate mounts never include this runtime directory.
	resultPath := filepath.Join(filepath.Dir(path), "result.json")
	encoded, _ := json.Marshal(report)
	rf, e := os.OpenFile(resultPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return 2
	}
	_, e = rf.Write(encoded)
	if e == nil {
		e = rf.Sync()
	}
	closeErr := rf.Close()
	if e != nil || closeErr != nil {
		return 2
	}
	if report.State == "blocked" {
		return 3
	}
	if report.Outcome != gotestprofile.OutcomePassed {
		return 1
	}
	return 0
}
