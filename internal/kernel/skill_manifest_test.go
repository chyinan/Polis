package kernel

import (
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestParseSkillDocumentAcceptsPinnedMetadataWithoutExecution(t *testing.T) {
	document := []byte("---\nname: Review Notes\ndescription: Read the checklist\nallowed-tools: [polis.read]\n---\n# Body\n")
	parsed, err := ParseSkillDocument(document, true)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.PackageID != "review-notes" || parsed.DisplayName != "Review Notes" || !strings.HasPrefix(parsed.ContentDigest, "sha256:") {
		t.Fatalf("parsed identity = %+v", parsed)
	}
	if !strings.Contains(string(parsed.Manifest), `"executionStatus":"unverified"`) || !strings.Contains(string(parsed.Manifest), `"scriptsExecuted":false`) || !strings.Contains(string(parsed.Manifest), `"scriptsPresent":true`) {
		t.Fatalf("manifest = %s", parsed.Manifest)
	}
	again, err := ParseSkillDocument(document, true)
	if err != nil || again.ContentDigest != parsed.ContentDigest {
		t.Fatalf("digest changed: %v %s %s", err, again.ContentDigest, parsed.ContentDigest)
	}
}

func TestParseSkillDocumentRejectsUnsafeFrontMatter(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{name: "alias", body: "---\nname: demo\nother: &a\n---\n", want: core.Malformed},
		{name: "duplicate", body: "---\nname: demo\nname: other\n---\n", want: core.Malformed},
		{name: "secret", body: "---\nname: demo\napi_key: hidden\n---\n", want: core.Denied},
		{name: "missing name", body: "---\ndescription: only\n---\n", want: core.Malformed},
		{name: "unknown", body: "---\nname: demo\nmetadata: extra\n---\n", want: core.Malformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSkillDocument([]byte(tc.body), false)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	_, err := ParseSkillDocument([]byte(strings.Repeat("a", maxManifestDocument+1)), false)
	if !errors.Is(err, core.TooLarge) {
		t.Fatalf("oversized err = %v", err)
	}
}

func TestParseMCPDocumentStoresDefinitionWithoutConnecting(t *testing.T) {
	parsed, err := ParseMCPDocument([]byte("name: Docs\ntransport: stdio\ncommand: echo\nargs: [hello]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != "docs" || parsed.Transport != "stdio" || parsed.Command == nil || *parsed.Command != "echo" || parsed.Endpoint != nil || !strings.HasPrefix(parsed.DescriptorDigest, "sha256:") {
		t.Fatalf("parsed = %+v", parsed)
	}
	_, err = ParseMCPDocument([]byte("name: Docs\ntransport: streamable_http\nendpoint: http://user:pass@example.test/mcp\n"))
	if !errors.Is(err, core.Malformed) {
		t.Fatalf("userinfo err = %v", err)
	}
	_, err = ParseMCPDocument([]byte("name: Docs\ntransport: stdio\ncommand: echo\ntoken: secret\n"))
	if !errors.Is(err, core.Denied) {
		t.Fatalf("secret err = %v", err)
	}
}
