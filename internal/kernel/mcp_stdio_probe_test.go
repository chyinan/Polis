package kernel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestMain(m *testing.M) {
	if os.Getenv("POLIS_MCP_FIXTURE") == "1" {
		if err := runProbeFixture(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestQualifyStdioMCPListsToolsWithoutCallingThem(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := qualifyStdioMCP(context.Background(), executable, []string{"-test.run=^$"}, []string{"POLIS_MCP_FIXTURE=1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ToolsCalled != 0 || result.ServerName != "polis-fixture" || len(result.ToolNames) != 1 || result.ToolNames[0] != "polis_probe_ping" || result.NetworkRestricted || len(result.ResourceSummaries) != 1 || result.ResourceSummaries[0].URI != "polis://probe/ping" {
		t.Fatalf("probe = %+v", result)
	}
}

func TestExperimentBrandGuidelinesAndMemoryServer(t *testing.T) {
	if os.Getenv("POLIS_CAPABILITY_EXPERIMENT") != "1" {
		t.Skip()
	}
	root := os.Getenv("POLIS_EXPERIMENT_SKILL_ROOT")
	script := os.Getenv("POLIS_EXPERIMENT_MCP_SCRIPT")
	scanned, err := ReadSkillPackage(root, "brand-guidelines")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSkillDocument(scanned.Document, scanned.ScriptsPresent)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := QualifyStdioMCP(context.Background(), "node", []string{script})
	if err != nil {
		t.Fatal(err)
	}
	if probe.ToolsCalled != 0 || len(probe.ToolNames) == 0 {
		t.Fatalf("memory probe = %+v", probe)
	}
	evidence := map[string]any{
		"skill": map[string]any{"packageId": parsed.PackageID, "displayName": parsed.DisplayName, "sourceRef": scanned.SourceRef, "contentDigest": parsed.ContentDigest, "scriptsPresent": scanned.ScriptsPresent, "executed": false},
		"mcp":   map[string]any{"package": "@modelcontextprotocol/server-memory@2.0.1", "protocolVersion": probe.ProtocolVersion, "serverName": probe.ServerName, "serverVersion": probe.ServerVersion, "toolNames": probe.ToolNames, "descriptorDigest": probe.DescriptorDigest, "toolsCalled": probe.ToolsCalled, "networkRestricted": probe.NetworkRestricted},
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join("..", "..", "evidence", "development", "r0.10-local-capability-probe", "result.json")
	if err = os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(destination, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("skill=%s mcp=%s tools=%v", parsed.PackageID, probe.ServerName, probe.ToolNames)
}

func TestReadSkillPackageRejectsSiblingScan(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "brand-guidelines")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	document := []byte("---\nname: brand-guidelines\ndescription: colors\n---\nUse the palette.\n")
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), document, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "palette.md"), []byte("colors"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanned, err := ReadSkillPackage(root, "brand-guidelines")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseSkillDocument(scanned.Document, scanned.ScriptsPresent, scanned.References)
	if err != nil || parsed.PackageID != "brand-guidelines" || parsed.ContentDigest == "" || !strings.Contains(string(parsed.Manifest), `"references":["references/palette.md"]`) {
		t.Fatalf("parsed = %+v %v", parsed, err)
	}
	if _, err = ReadSkillPackage(root, "../brand-guidelines"); !errors.Is(err, core.Malformed) {
		t.Fatalf("escape err = %v", err)
	}
}

func runProbeFixture(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readMCPFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(body, &request) != nil {
			return errors.New("malformed fixture request")
		}
		if len(request.ID) == 0 {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2024-11-05", "serverInfo": map[string]string{"name": "polis-fixture", "version": "0"}, "capabilities": map[string]any{}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "polis_probe_ping", "inputSchema": map[string]any{"type": "object"}}}}
		case "resources/list":
			result = map[string]any{"resources": []any{map[string]any{"uri": "polis://probe/ping", "name": "Probe ping"}}}
		default:
			return errors.New("unexpected method " + request.Method)
		}
		if err = writeMCP(out, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			return err
		}
	}
}
