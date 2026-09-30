// pattern: Functional Core
package capabilitysource

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
)

type testZIPFile struct {
	name    string
	content string
}

func TestPrepareReadOnlySkillBundleBuildsCanonicalDigestManifest(t *testing.T) {
	archive := skillZIP(t, []testZIPFile{
		{name: "SKILL.md", content: "---\nname: design-review\ndescription: Review designs using read-only references.\nlicense: CC0\nallowed-tools: Read\n---\n# Design review\nUse cited sources and report uncertainty.\n"},
		{name: "README.md", content: "# Package notes\n"},
		{name: "references/checklist.md", content: "# Checklist\nCheck contrast and keyboard access.\n"},
		{name: "assets/diagram.txt", content: "diagram notes"},
	})

	bundle, err := PrepareReadOnlySkillBundle(archive)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest.SchemaVersion != ReadOnlySkillBundleSchema || bundle.Manifest.Name != "design-review" || bundle.Manifest.Description != "Review designs using read-only references." || bundle.ContentDigest == "" {
		t.Fatalf("parsed Skill bundle metadata is incomplete: %+v", bundle.Manifest)
	}
	if len(bundle.Files) != 4 || bundle.Files[0].RelativePath != "README.md" || bundle.Files[1].RelativePath != "SKILL.md" || bundle.Files[2].RelativePath != "assets/diagram.txt" || bundle.Files[3].RelativePath != "references/checklist.md" {
		t.Fatalf("Skill bundle files are not canonical and sorted: %+v", bundle.Files)
	}
	if err = VerifyReadOnlySkillBundle(bundle.Manifest, bundle.ContentDigest, bundle.Files); err != nil {
		t.Fatalf("generated Skill source manifest did not verify: %v", err)
	}
	if bytes.Equal(bundle.ManifestJSON, archive) {
		t.Fatal("package manifest digest unexpectedly aliases the uploaded ZIP bytes")
	}
}

func TestReadOnlySkillDigestIgnoresZIPEntryOrder(t *testing.T) {
	skill := "---\nname: stable-package\ndescription: A stable package.\n---\nRead only.\n"
	first := skillZIP(t, []testZIPFile{{name: "SKILL.md", content: skill}, {name: "references/one.md", content: "one"}})
	second := skillZIP(t, []testZIPFile{{name: "references/one.md", content: "one"}, {name: "SKILL.md", content: skill}})
	firstBundle, err := PrepareReadOnlySkillBundle(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBundle, err := PrepareReadOnlySkillBundle(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstBundle.ContentDigest != secondBundle.ContentDigest || !bytes.Equal(firstBundle.ManifestJSON, secondBundle.ManifestJSON) {
		t.Fatalf("same Skill contents had order-dependent digests: %s != %s", firstBundle.ContentDigest, secondBundle.ContentDigest)
	}
}

func TestReadOnlySkillBundleRejectsUnsupportedOrMalformedSources(t *testing.T) {
	validSkill := "---\nname: safe-skill\ndescription: A safe skill.\n---\nRead only.\n"
	cases := []struct {
		name  string
		files []testZIPFile
	}{
		{name: "missing SKILL.md", files: []testZIPFile{{name: "references/readme.md", content: "reference"}}},
		{name: "script source", files: []testZIPFile{{name: "SKILL.md", content: validSkill}, {name: "scripts/run.py", content: "print('never run')"}}},
		{name: "active SVG attachment", files: []testZIPFile{{name: "SKILL.md", content: validSkill}, {name: "assets/diagram.svg", content: "<svg></svg>"}}},
		{name: "duplicate frontmatter key", files: []testZIPFile{{name: "SKILL.md", content: "---\nname: safe-skill\nname: other\ndescription: A safe skill.\n---\nBody.\n"}}},
		{name: "unknown frontmatter key", files: []testZIPFile{{name: "SKILL.md", content: "---\nname: safe-skill\ndescription: A safe skill.\npermissions: administrator\n---\nBody.\n"}}},
		{name: "YAML alias", files: []testZIPFile{{name: "SKILL.md", content: "---\nname: &n safe-skill\ndescription: *n\n---\nBody.\n"}}},
		{name: "custom YAML tag", files: []testZIPFile{{name: "SKILL.md", content: "---\nname: !unsafe safe-skill\ndescription: A safe skill.\n---\nBody.\n"}}},
		{name: "invalid Skill name", files: []testZIPFile{{name: "SKILL.md", content: "---\nname: Bad_Name\ndescription: A safe skill.\n---\nBody.\n"}}},
		{name: "multiple roots", files: []testZIPFile{{name: "SKILL.md", content: validSkill}, {name: "other/README.md", content: "outside package"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := PrepareReadOnlySkillBundle(skillZIP(t, testCase.files)); err == nil {
				t.Fatal("unsafe or malformed Skill package was accepted")
			}
		})
	}
}

func TestVerifyReadOnlySkillBundleRejectsChangedOrMissingCASFiles(t *testing.T) {
	bundle, err := PrepareReadOnlySkillBundle(skillZIP(t, []testZIPFile{{name: "SKILL.md", content: "---\nname: check-source\ndescription: Verify file digests.\n---\nBody.\n"}, {name: "references/check.md", content: "verified"}}))
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]SkillBundleFile(nil), bundle.Files...)
	changed[0].Content = append([]byte(nil), changed[0].Content...)
	changed[0].Content[0] ^= 1
	if err = VerifyReadOnlySkillBundle(bundle.Manifest, bundle.ContentDigest, changed); err == nil {
		t.Fatal("changed CAS source bytes passed the Skill digest check")
	}
	if err = VerifyReadOnlySkillBundle(bundle.Manifest, bundle.ContentDigest, bundle.Files[:1]); err == nil {
		t.Fatal("missing CAS source file passed the Skill digest check")
	}
	duplicate := bundle.Manifest
	duplicate.Files = append(append([]SkillBundleFileManifest(nil), bundle.Manifest.Files...), bundle.Manifest.Files[0])
	duplicateJSON, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	duplicateDigest := digestBytes(duplicateJSON)
	duplicateFiles := append(append([]SkillBundleFile(nil), bundle.Files...), bundle.Files[0])
	if err = VerifyReadOnlySkillBundle(duplicate, duplicateDigest, duplicateFiles); err == nil {
		t.Fatal("duplicate package paths passed the Skill manifest check")
	}
}

func skillZIP(t *testing.T, files []testZIPFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, err := writer.Create(file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
