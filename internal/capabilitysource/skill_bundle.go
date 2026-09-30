// pattern: Functional Core
package capabilitysource

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"polis/internal/intake"
)

const (
	ReadOnlySkillBundleSchema = "polis-read-only-skill@1"
	maxSkillFrontmatterBytes  = 8 << 10
	maxSkillMarkdownBytes     = 64 << 10
	maxSkillAssetBytes        = 1 << 20
	maxSkillYAMLNodes         = 512
	maxSkillYAMLDepth         = 16
)

var (
	ErrInvalidReadOnlySkill = errors.New("invalid read-only Skill package")
	skillNamePattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type SkillBundleFileManifest struct {
	RelativePath  string `json:"relativePath"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentSHA256 string `json:"contentSHA256"`
}

type ReadOnlySkillManifest struct {
	SchemaVersion string                    `json:"schemaVersion"`
	ReadOnly      bool                      `json:"readOnly"`
	Name          string                    `json:"name"`
	Description   string                    `json:"description"`
	Files         []SkillBundleFileManifest `json:"files"`
}

type SkillBundleFile struct {
	RelativePath  string
	MediaType     string
	ByteSize      int64
	ContentSHA256 string
	Content       []byte
}

type ReadOnlySkillBundle struct {
	Manifest      ReadOnlySkillManifest
	ManifestJSON  []byte
	ContentDigest string
	Files         []SkillBundleFile
}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	AllowedTools  yaml.Node         `yaml:"allowed-tools"`
	Metadata      map[string]string `yaml:"metadata"`
}

func PrepareReadOnlySkillBundle(archive []byte) (ReadOnlySkillBundle, error) {
	files, err := intake.ExtractVerifiedZIPFiles(archive)
	if err != nil {
		return ReadOnlySkillBundle{}, fmt.Errorf("%w: invalid ZIP source", ErrInvalidReadOnlySkill)
	}
	files, err = stripSkillPackageRoot(files)
	if err != nil {
		return ReadOnlySkillBundle{}, err
	}
	if len(files) == 0 || len(files) > intake.MaxDirectoryFiles {
		return ReadOnlySkillBundle{}, fmt.Errorf("%w: file count is outside the supported bound", ErrInvalidReadOnlySkill)
	}

	preparedFiles := make([]SkillBundleFile, 0, len(files))
	var totalBytes int64
	var skillMarkdown []byte
	for _, file := range files {
		if !skillPathAllowed(file.RelativePath) {
			return ReadOnlySkillBundle{}, fmt.Errorf("%w: unsupported package path", ErrInvalidReadOnlySkill)
		}
		prepared, prepareErr := prepareSkillFile(file.RelativePath, file.Content)
		if prepareErr != nil || prepared.State != intake.StateUsable {
			return ReadOnlySkillBundle{}, fmt.Errorf("%w: unsupported or invalid file", ErrInvalidReadOnlySkill)
		}
		if path.Ext(file.RelativePath) == ".md" || path.Ext(file.RelativePath) == ".txt" {
			if !utf8.Valid(file.Content) || bytes.IndexByte(file.Content, 0) >= 0 {
				return ReadOnlySkillBundle{}, fmt.Errorf("%w: text file is not valid UTF-8", ErrInvalidReadOnlySkill)
			}
		}
		if file.RelativePath == "SKILL.md" {
			if len(file.Content) > maxSkillMarkdownBytes {
				return ReadOnlySkillBundle{}, fmt.Errorf("%w: SKILL.md exceeds its size bound", ErrInvalidReadOnlySkill)
			}
			skillMarkdown = file.Content
		} else if len(file.Content) > maxSkillAssetBytes {
			return ReadOnlySkillBundle{}, fmt.Errorf("%w: attachment exceeds its size bound", ErrInvalidReadOnlySkill)
		}
		totalBytes += int64(len(file.Content))
		if totalBytes > intake.MaxDirectoryBytes {
			return ReadOnlySkillBundle{}, fmt.Errorf("%w: package exceeds its expanded-size bound", ErrInvalidReadOnlySkill)
		}
		copyOfContent := append([]byte(nil), file.Content...)
		preparedFiles = append(preparedFiles, SkillBundleFile{
			RelativePath: file.RelativePath, MediaType: prepared.MediaType, ByteSize: int64(len(copyOfContent)),
			ContentSHA256: digestBytes(copyOfContent), Content: copyOfContent,
		})
	}
	if skillMarkdown == nil {
		return ReadOnlySkillBundle{}, fmt.Errorf("%w: SKILL.md is required at the package root", ErrInvalidReadOnlySkill)
	}
	name, description, err := parseSkillFrontmatter(skillMarkdown)
	if err != nil {
		return ReadOnlySkillBundle{}, err
	}
	sort.Slice(preparedFiles, func(i, j int) bool { return preparedFiles[i].RelativePath < preparedFiles[j].RelativePath })
	manifest := ReadOnlySkillManifest{
		SchemaVersion: ReadOnlySkillBundleSchema,
		ReadOnly:      true,
		Name:          name,
		Description:   description,
		Files:         skillBundleFileManifest(preparedFiles),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return ReadOnlySkillBundle{}, fmt.Errorf("%w: cannot encode canonical manifest", ErrInvalidReadOnlySkill)
	}
	return ReadOnlySkillBundle{
		Manifest: manifest, ManifestJSON: manifestJSON, ContentDigest: digestBytes(manifestJSON), Files: preparedFiles,
	}, nil
}

func VerifyReadOnlySkillBundle(manifest ReadOnlySkillManifest, contentDigest string, files []SkillBundleFile) error {
	if manifest.SchemaVersion != ReadOnlySkillBundleSchema || !manifest.ReadOnly || !validSkillName(manifest.Name) || !validSkillDescription(manifest.Description) || len(manifest.Files) == 0 || len(manifest.Files) > intake.MaxDirectoryFiles || len(files) != len(manifest.Files) || !sort.SliceIsSorted(manifest.Files, func(i, j int) bool { return manifest.Files[i].RelativePath < manifest.Files[j].RelativePath }) {
		return fmt.Errorf("%w: invalid manifest shape", ErrInvalidReadOnlySkill)
	}
	verifiedFiles := append([]SkillBundleFile(nil), files...)
	sort.Slice(verifiedFiles, func(i, j int) bool { return verifiedFiles[i].RelativePath < verifiedFiles[j].RelativePath })
	seenPaths := make(map[string]struct{}, len(verifiedFiles))
	var totalBytes int64
	var skillMarkdown []byte
	for index, file := range verifiedFiles {
		entry := manifest.Files[index]
		prepared, err := prepareSkillFile(file.RelativePath, file.Content)
		if err != nil || prepared.State != intake.StateUsable || prepared.MediaType != file.MediaType || file.RelativePath != entry.RelativePath || file.MediaType != entry.MediaType || file.ByteSize != entry.ByteSize || file.ContentSHA256 != entry.ContentSHA256 || !skillPathAllowed(file.RelativePath) || file.ByteSize != int64(len(file.Content)) || file.ByteSize <= 0 || file.ContentSHA256 != digestBytes(file.Content) {
			return fmt.Errorf("%w: file manifest does not match CAS content", ErrInvalidReadOnlySkill)
		}
		if _, exists := seenPaths[file.RelativePath]; exists {
			return fmt.Errorf("%w: duplicate package path", ErrInvalidReadOnlySkill)
		}
		seenPaths[file.RelativePath] = struct{}{}
		totalBytes += file.ByteSize
		if totalBytes > intake.MaxDirectoryBytes {
			return fmt.Errorf("%w: package exceeds its expanded-size bound", ErrInvalidReadOnlySkill)
		}
		if file.RelativePath == "SKILL.md" {
			skillMarkdown = file.Content
		}
	}
	if skillMarkdown == nil {
		return fmt.Errorf("%w: SKILL.md is missing", ErrInvalidReadOnlySkill)
	}
	name, description, err := parseSkillFrontmatter(skillMarkdown)
	if err != nil || name != manifest.Name || description != manifest.Description {
		return fmt.Errorf("%w: frontmatter does not match manifest", ErrInvalidReadOnlySkill)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil || digestBytes(manifestJSON) != contentDigest {
		return fmt.Errorf("%w: canonical content digest mismatch", ErrInvalidReadOnlySkill)
	}
	return nil
}

func stripSkillPackageRoot(files []intake.DirectoryInputFile) ([]intake.DirectoryInputFile, error) {
	for _, file := range files {
		if file.RelativePath == "SKILL.md" {
			return files, nil
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: ZIP contains no files", ErrInvalidReadOnlySkill)
	}
	root, _, hasRootPath := strings.Cut(files[0].RelativePath, "/")
	if root == "" || !hasRootPath || strings.HasPrefix(root, ".") {
		return nil, fmt.Errorf("%w: ZIP must contain one package root", ErrInvalidReadOnlySkill)
	}
	normalized := make([]intake.DirectoryInputFile, 0, len(files))
	for _, file := range files {
		prefix := root + "/"
		if !strings.HasPrefix(file.RelativePath, prefix) {
			return nil, fmt.Errorf("%w: ZIP has multiple package roots", ErrInvalidReadOnlySkill)
		}
		relativePath := strings.TrimPrefix(file.RelativePath, prefix)
		if relativePath == "" || path.Clean(relativePath) != relativePath || path.IsAbs(relativePath) {
			return nil, fmt.Errorf("%w: invalid package-relative path", ErrInvalidReadOnlySkill)
		}
		file.RelativePath = relativePath
		normalized = append(normalized, file)
	}
	return normalized, nil
}

func skillPathAllowed(relativePath string) bool {
	for _, segment := range strings.Split(relativePath, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	if relativePath == "SKILL.md" || relativePath == "README.md" || relativePath == "LICENSE" || relativePath == "LICENSE.txt" || relativePath == "LICENSE.md" {
		return true
	}
	if strings.HasPrefix(relativePath, "references/") {
		extension := strings.ToLower(path.Ext(relativePath))
		return extension == ".md" || extension == ".txt"
	}
	if strings.HasPrefix(relativePath, "assets/") {
		extension := strings.ToLower(path.Ext(relativePath))
		return extension == ".md" || extension == ".txt" || extension == ".png" || extension == ".jpg" || extension == ".jpeg"
	}
	return false
}

func prepareSkillFile(relativePath string, content []byte) (intake.PreparedUpload, error) {
	if relativePath == "LICENSE" {
		if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
			return intake.PreparedUpload{}, ErrInvalidReadOnlySkill
		}
		return intake.PrepareUpload("LICENSE.txt", "text/plain", content)
	}
	return intake.PrepareUpload(path.Base(relativePath), "application/octet-stream", content)
}

func parseSkillFrontmatter(content []byte) (string, string, error) {
	if len(content) == 0 || !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return "", "", fmt.Errorf("%w: SKILL.md is not valid UTF-8 text", ErrInvalidReadOnlySkill)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if strings.HasPrefix(text, "\uFEFF") || !strings.HasPrefix(text, "---\n") {
		return "", "", fmt.Errorf("%w: SKILL.md needs a YAML frontmatter block", ErrInvalidReadOnlySkill)
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 || end+4 > maxSkillFrontmatterBytes {
		return "", "", fmt.Errorf("%w: frontmatter is missing or too large", ErrInvalidReadOnlySkill)
	}
	frontmatterBytes := []byte(text[4 : 4+end])
	bodyStart := 4 + end + len("\n---\n")
	if strings.TrimSpace(text[bodyStart:]) == "" {
		return "", "", fmt.Errorf("%w: Skill instructions are empty", ErrInvalidReadOnlySkill)
	}
	var frontmatter skillFrontmatter
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatterBytes))
	decoder.KnownFields(true)
	if err := decoder.Decode(&frontmatter); err != nil {
		return "", "", fmt.Errorf("%w: invalid YAML frontmatter", ErrInvalidReadOnlySkill)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", "", fmt.Errorf("%w: frontmatter contains multiple YAML documents", ErrInvalidReadOnlySkill)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(frontmatterBytes, &document); err != nil || validateSkillYAMLNode(&document, 0, new(int)) != nil || validateSkillFrontmatterNode(&document) != nil {
		return "", "", fmt.Errorf("%w: frontmatter uses unsupported YAML features", ErrInvalidReadOnlySkill)
	}
	if !validSkillName(frontmatter.Name) || !validSkillDescription(frontmatter.Description) {
		return "", "", fmt.Errorf("%w: name or description is invalid", ErrInvalidReadOnlySkill)
	}
	if len(frontmatter.License) > 256 || len(frontmatter.Compatibility) > 512 || !validSkillAllowedTools(frontmatter.AllowedTools) || !validSkillMetadata(frontmatter.Metadata) {
		return "", "", fmt.Errorf("%w: optional frontmatter is invalid", ErrInvalidReadOnlySkill)
	}
	return frontmatter.Name, strings.TrimSpace(frontmatter.Description), nil
}

func validateSkillFrontmatterNode(document *yaml.Node) error {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return ErrInvalidReadOnlySkill
	}
	root := document.Content[0]
	for index := 0; index < len(root.Content); index += 2 {
		key := root.Content[index].Value
		value := root.Content[index+1]
		switch key {
		case "name", "description", "license", "compatibility":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return ErrInvalidReadOnlySkill
			}
		case "allowed-tools":
			if value.Kind == yaml.ScalarNode {
				if value.Tag != "!!str" {
					return ErrInvalidReadOnlySkill
				}
				continue
			}
			if value.Kind != yaml.SequenceNode {
				return ErrInvalidReadOnlySkill
			}
			for _, item := range value.Content {
				if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
					return ErrInvalidReadOnlySkill
				}
			}
		case "metadata":
			if value.Kind != yaml.MappingNode {
				return ErrInvalidReadOnlySkill
			}
			for metadataIndex := 0; metadataIndex < len(value.Content); metadataIndex += 2 {
				metadataKey, metadataValue := value.Content[metadataIndex], value.Content[metadataIndex+1]
				if metadataKey.Kind != yaml.ScalarNode || metadataKey.Tag != "!!str" || metadataValue.Kind != yaml.ScalarNode || metadataValue.Tag != "!!str" {
					return ErrInvalidReadOnlySkill
				}
			}
		default:
			return ErrInvalidReadOnlySkill
		}
	}
	return nil
}

func validateSkillYAMLNode(node *yaml.Node, depth int, count *int) error {
	if node == nil || depth > maxSkillYAMLDepth {
		return ErrInvalidReadOnlySkill
	}
	*count++
	if *count > maxSkillYAMLNodes || node.Anchor != "" || node.Kind == yaml.AliasNode || node.Alias != nil {
		return ErrInvalidReadOnlySkill
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			return ErrInvalidReadOnlySkill
		}
	case yaml.MappingNode:
		if node.Tag != "!!map" || len(node.Content)%2 != 0 {
			return ErrInvalidReadOnlySkill
		}
		keys := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return ErrInvalidReadOnlySkill
			}
			if _, exists := keys[key.Value]; exists {
				return ErrInvalidReadOnlySkill
			}
			keys[key.Value] = struct{}{}
		}
	case yaml.SequenceNode:
		if node.Tag != "!!seq" {
			return ErrInvalidReadOnlySkill
		}
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str", "!!null", "!!bool", "!!int", "!!float":
		default:
			return ErrInvalidReadOnlySkill
		}
	default:
		return ErrInvalidReadOnlySkill
	}
	for _, child := range node.Content {
		if err := validateSkillYAMLNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func validSkillAllowedTools(node yaml.Node) bool {
	if node.IsZero() {
		return true
	}
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Tag == "!!str" && len(node.Value) <= 1024
	case yaml.SequenceNode:
		if len(node.Content) > 64 {
			return false
		}
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || len(item.Value) > 256 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validSkillMetadata(metadata map[string]string) bool {
	if len(metadata) > 32 {
		return false
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || len(key) > 128 || len(value) > 1024 {
			return false
		}
	}
	return true
}

func validSkillName(name string) bool {
	return len(name) >= 1 && len(name) <= 64 && skillNamePattern.MatchString(name)
}

func validSkillDescription(description string) bool {
	return strings.TrimSpace(description) != "" && utf8.ValidString(description) && !strings.ContainsRune(description, '\x00') && len(description) <= 1024
}

func skillBundleFileManifest(files []SkillBundleFile) []SkillBundleFileManifest {
	manifest := make([]SkillBundleFileManifest, 0, len(files))
	for _, file := range files {
		manifest = append(manifest, SkillBundleFileManifest{RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize, ContentSHA256: file.ContentSHA256})
	}
	return manifest
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
