// pattern: Imperative Shell
package kernel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"polis/internal/core"
)

const maxLibraryEntries = 32

type ScannedSkill struct {
	SourceRef      string
	Document       []byte
	ScriptsPresent bool
	References     []SkillReferenceFile
}

type ScannedMCP struct {
	SourceRef string
	Document  []byte
}

type ScanRejection struct {
	SourceRef string `json:"sourceRef"`
	Reason    string `json:"reason"`
}

type LibraryScan struct {
	Skills   []ScannedSkill
	MCP      []ScannedMCP
	Rejected []ScanRejection
}

func ReadSkillPackage(root, packageName string) (ScannedSkill, error) {
	cleaned, err := libraryRoot(root)
	if err != nil {
		return ScannedSkill{}, err
	}
	if !safePathSegment(packageName) {
		return ScannedSkill{}, core.Malformed
	}
	dir := filepath.Join(cleaned, "skills", packageName)
	link, err := isSymlink(dir)
	if err != nil {
		return ScannedSkill{}, core.OutOfScope
	}
	if link {
		return ScannedSkill{}, core.Denied
	}
	documentPath := filepath.Join(dir, "SKILL.md")
	if !pathInside(cleaned, documentPath) {
		return ScannedSkill{}, core.Denied
	}
	document, reason := readRegularFile(documentPath)
	if reason != "" {
		return ScannedSkill{}, core.Malformed
	}
	references, err := referenceFiles(dir)
	if err != nil {
		return ScannedSkill{}, err
	}
	return ScannedSkill{SourceRef: "library://skills/" + packageName + "/SKILL.md", Document: document, ScriptsPresent: regularDirectoryExists(filepath.Join(dir, "scripts")), References: references}, nil
}

func referenceFiles(skillDir string) ([]SkillReferenceFile, error) {
	dir := filepath.Join(skillDir, "references")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []SkillReferenceFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []SkillReferenceFile{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !safeReferenceName(name) {
			continue
		}
		path := filepath.Join(dir, name)
		link, linkErr := isSymlink(path)
		if linkErr != nil || link || !pathInside(skillDir, path) {
			continue
		}
		body, reason := readRegularFile(path)
		if reason == "too_large" {
			return nil, core.TooLarge
		}
		if reason != "" {
			return nil, core.Malformed
		}
		files = append(files, SkillReferenceFile{Name: "references/" + name, Body: body})
		if len(files) > maxManifestListItems {
			return nil, core.TooLarge
		}
	}
	return files, nil
}

func safeReferenceName(name string) bool {
	return name != "" && name != "." && name != ".." && strings.HasSuffix(name, ".md") && !strings.ContainsAny(name, `/\`) && len(name) <= 80
}

func ScanSkillLibrary(root string) (LibraryScan, error) {
	cleaned, err := libraryRoot(root)
	if err != nil {
		return LibraryScan{}, err
	}
	var scan LibraryScan
	skillsDir := filepath.Join(cleaned, "skills")
	if err = scanPackageDir(cleaned, skillsDir, &scan); err != nil {
		return LibraryScan{}, err
	}
	mcpDir := filepath.Join(cleaned, "mcp")
	if err = scanMCPDir(cleaned, mcpDir, &scan); err != nil {
		return LibraryScan{}, err
	}
	if scan.Skills == nil {
		scan.Skills = []ScannedSkill{}
	}
	if scan.MCP == nil {
		scan.MCP = []ScannedMCP{}
	}
	if scan.Rejected == nil {
		scan.Rejected = []ScanRejection{}
	}
	return scan, nil
}

func libraryRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", core.Denied
	}
	cleaned := filepath.Clean(root)
	if !filepath.IsAbs(cleaned) {
		return "", core.Malformed
	}
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", core.OutOfScope
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", core.Denied
	}
	return cleaned, nil
}

func scanPackageDir(root, skillsDir string, scan *LibraryScan) error {
	entries, err := os.ReadDir(skillsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > maxLibraryEntries {
		return core.TooLarge
	}
	for _, entry := range entries {
		name := entry.Name()
		source := "library://skills/" + name + "/SKILL.md"
		if !safePathSegment(name) {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "unsupported_entry"})
			continue
		}
		dir := filepath.Join(skillsDir, name)
		if link, linkErr := isSymlink(dir); linkErr != nil || link {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "symlink"})
			continue
		}
		if !entry.IsDir() {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "unsupported_entry"})
			continue
		}
		documentPath := filepath.Join(dir, "SKILL.md")
		if !pathInside(root, documentPath) {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "escape"})
			continue
		}
		document, reason := readRegularFile(documentPath)
		if reason != "" {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: reason})
			continue
		}
		scriptsPresent := regularDirectoryExists(filepath.Join(dir, "scripts"))
		references, refErr := referenceFiles(dir)
		if refErr != nil {
			reason := "references"
			if errors.Is(refErr, core.TooLarge) {
				reason = "too_large"
			}
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: reason})
			continue
		}
		scan.Skills = append(scan.Skills, ScannedSkill{SourceRef: source, Document: document, ScriptsPresent: scriptsPresent, References: references})
	}
	return nil
}

func scanMCPDir(root, mcpDir string, scan *LibraryScan) error {
	entries, err := os.ReadDir(mcpDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > maxLibraryEntries {
		return core.TooLarge
	}
	for _, entry := range entries {
		name := entry.Name()
		source := "library://mcp/" + name
		if entry.IsDir() || !strings.HasSuffix(name, ".yaml") || !safePathSegment(strings.TrimSuffix(name, ".yaml")) {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "unsupported_entry"})
			continue
		}
		documentPath := filepath.Join(mcpDir, name)
		if link, linkErr := isSymlink(documentPath); linkErr != nil || link || !pathInside(root, documentPath) {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: "symlink"})
			continue
		}
		document, reason := readRegularFile(documentPath)
		if reason != "" {
			scan.Rejected = append(scan.Rejected, ScanRejection{SourceRef: source, Reason: reason})
			continue
		}
		scan.MCP = append(scan.MCP, ScannedMCP{SourceRef: source, Document: document})
	}
	return nil
}

func safePathSegment(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`) && core.ValidID(name)
}

func pathInside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

func readRegularFile(path string) ([]byte, string) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "missing"
	}
	if err != nil {
		return nil, "unreadable"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, "symlink"
	}
	if !info.Mode().IsRegular() {
		return nil, "not_regular"
	}
	if info.Size() > maxManifestDocument {
		return nil, "too_large"
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, "unreadable"
	}
	if len(document) > maxManifestDocument {
		return nil, "too_large"
	}
	return document, ""
}

func regularDirectoryExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}
