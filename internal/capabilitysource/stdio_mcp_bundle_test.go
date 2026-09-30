// pattern: Functional Core
package capabilitysource

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

func TestPrepareStdioMCPBundlePinsCanonicalPackageFiles(t *testing.T) {
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`)
	contents := map[string][]byte{
		"runtime/node.exe":  []byte("node-runtime-fixture"),
		"server/main.mjs":   []byte("import './helper.mjs';\n"),
		"server/helper.mjs": []byte("export const value = 1;\n"),
	}
	orders := [][]string{
		{"runtime/node.exe", "server/main.mjs", "server/helper.mjs"},
		{"runtime/node.exe", "server/helper.mjs", "server/main.mjs"},
		{"server/main.mjs", "runtime/node.exe", "server/helper.mjs"},
		{"server/main.mjs", "server/helper.mjs", "runtime/node.exe"},
		{"server/helper.mjs", "runtime/node.exe", "server/main.mjs"},
		{"server/helper.mjs", "server/main.mjs", "runtime/node.exe"},
	}
	var first StdioMCPBundle
	for index, order := range orders {
		entries := make([]stdioMCPArchiveEntry, 0, len(order))
		for _, name := range order {
			entries = append(entries, stdioMCPArchiveEntry{name: name, content: contents[name]})
		}
		bundle, err := PrepareStdioMCPBundle(stdioMCPArchiveOrdered(t, manifest, entries))
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			first = bundle
		}
		if bundle.ManifestDigest != first.ManifestDigest || bundle.ManifestDigest != bundle.ContentDigest {
			t.Fatalf("package digest varies with ZIP order %v: first=%s got=%s content=%s", order, first.ManifestDigest, bundle.ManifestDigest, bundle.ContentDigest)
		}
	}
	if first.ManifestDigest == "" {
		t.Fatal("canonical package digest is empty")
	}
	if first.Manifest.Name != "fixture-mcp" || first.Manifest.ServerName != "fixture-server" || first.Manifest.ServerVersion != "1.0.0" || first.Manifest.Command != "runtime/node.exe" || first.Manifest.EntryPoint != "server/main.mjs" {
		t.Fatalf("package metadata=%+v", first.Manifest)
	}
	if len(first.Files) != 3 || !sort.SliceIsSorted(first.Manifest.Files, func(i, j int) bool {
		return first.Manifest.Files[i].RelativePath < first.Manifest.Files[j].RelativePath
	}) {
		t.Fatalf("package files=%+v", first.Manifest.Files)
	}
	if err := VerifyStdioMCPBundle(first.Manifest, first.ManifestDigest, first.Files); err != nil {
		t.Fatalf("verify exact package bytes: %v", err)
	}
	first.Files[0].Content[0] ^= 0xff
	if err := VerifyStdioMCPBundle(first.Manifest, first.ManifestDigest, first.Files); !errors.Is(err, ErrInvalidStdioMCPBundle) {
		t.Fatalf("changed package byte error=%v, want invalid package", err)
	}
}

func TestPrepareStdioMCPBundleRejectsUnsafeManifestAndArchive(t *testing.T) {
	validFiles := map[string][]byte{"runtime/node.exe": []byte("runtime"), "server/main.mjs": []byte("server")}
	base := `"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]`
	cases := []struct {
		name     string
		manifest string
		files    map[string][]byte
	}{
		{name: "unknown field", manifest: `{` + base + `,"installScript":"npm install"}`, files: validFiles},
		{name: "traversal command", manifest: `{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"../node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`, files: validFiles},
		{name: "missing command file", manifest: `{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/missing.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`, files: validFiles},
		{name: "missing entry point", manifest: `{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/missing.mjs","args":["server/missing.mjs"]}`, files: validFiles},
		{name: "archive path escape", manifest: `{` + base + `}`, files: map[string][]byte{"runtime/node.exe": []byte("runtime"), "server/main.mjs": []byte("server"), "../outside.txt": []byte("escape")}},
		{name: "duplicate manifest", manifest: `{` + base + `}`, files: map[string][]byte{"runtime/node.exe": []byte("runtime"), "server/main.mjs": []byte("server"), "mcp-package.json": []byte("duplicate")}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := PrepareStdioMCPBundle(stdioMCPArchive(t, []byte(testCase.manifest), testCase.files)); !errors.Is(err, ErrInvalidStdioMCPBundle) {
				t.Fatalf("invalid package error=%v, want %s", err, ErrInvalidStdioMCPBundle)
			}
		})
	}
}

func TestPrepareStdioMCPBundleRejectsDuplicateJSONKeys(t *testing.T) {
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","name":"second-name","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`)
	archive := stdioMCPArchive(t, manifest, map[string][]byte{"runtime/node.exe": []byte("runtime"), "server/main.mjs": []byte("server")})
	if _, err := PrepareStdioMCPBundle(archive); !errors.Is(err, ErrInvalidStdioMCPBundle) {
		t.Fatalf("duplicate JSON key error=%v, want %s", err, ErrInvalidStdioMCPBundle)
	}
}

func TestPrepareStdioMCPBundleRejectsInvalidUTF8Manifest(t *testing.T) {
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture` + "\xff" + `-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`)
	archive := stdioMCPArchive(t, manifest, map[string][]byte{"runtime/node.exe": []byte("runtime"), "server/main.mjs": []byte("server")})
	if _, err := PrepareStdioMCPBundle(archive); !errors.Is(err, ErrInvalidStdioMCPBundle) {
		t.Fatalf("invalid UTF-8 manifest error=%v, want %s", err, ErrInvalidStdioMCPBundle)
	}
}

func TestPrepareStdioMCPBundleUsesSharedZIPPathByteLimit(t *testing.T) {
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"fixture-mcp","serverName":"fixture-server","serverVersion":"1.0.0","command":"runtime/node.exe","entryPoint":"server/main.mjs","args":["server/main.mjs"]}`)
	files := map[string][]byte{
		"runtime/node.exe": []byte("runtime"),
		"server/main.mjs":  []byte("server"),
		strings.Repeat("d/", 510) + "x.md": []byte("at-limit"),
	}
	if _, err := PrepareStdioMCPBundle(stdioMCPArchive(t, manifest, files)); err != nil {
		t.Fatalf("1024-byte package path was rejected: %v", err)
	}
	delete(files, strings.Repeat("d/", 510)+"x.md")
	files[strings.Repeat("d/", 510)+"x.mdx"] = []byte("over-limit")
	if _, err := PrepareStdioMCPBundle(stdioMCPArchive(t, manifest, files)); !errors.Is(err, ErrInvalidStdioMCPBundle) {
		t.Fatalf("1025-byte package path error=%v, want %s", err, ErrInvalidStdioMCPBundle)
	}
}

func TestPrepareStdioMCPBundleAllowsDescriptiveDisplayNames(t *testing.T) {
	manifest := []byte(`{"schemaVersion":"polis-controlled-stdio-mcp@1","name":"Internal data lookup","serverName":"fixture-server","serverVersion":"1.0.0","command":"server.exe","entryPoint":"server.exe","args":[]}`)
	bundle, err := PrepareStdioMCPBundle(stdioMCPArchive(t, manifest, map[string][]byte{"server.exe": []byte("server fixture")}))
	if err != nil {
		t.Fatalf("descriptive package name was rejected: %v", err)
	}
	if bundle.Manifest.Name != "Internal data lookup" {
		t.Fatalf("package display name=%q", bundle.Manifest.Name)
	}
}

func stdioMCPArchive(t *testing.T, manifest []byte, files map[string][]byte) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]stdioMCPArchiveEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, stdioMCPArchiveEntry{name: name, content: files[name]})
	}
	return stdioMCPArchiveOrdered(t, manifest, entries)
}

type stdioMCPArchiveEntry struct {
	name    string
	content []byte
}

func stdioMCPArchiveOrdered(t *testing.T, manifest []byte, files []stdioMCPArchiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	add := func(name string, contents []byte) {
		t.Helper()
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	add("mcp-package.json", manifest)
	for _, file := range files {
		add(file.name, file.content)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if json.Unmarshal(manifest, &decoded) != nil {
		t.Fatal("test manifest must be valid JSON")
	}
	return buffer.Bytes()
}
