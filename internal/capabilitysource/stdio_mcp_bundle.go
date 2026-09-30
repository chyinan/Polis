// pattern: Functional Core
package capabilitysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"polis/internal/intake"
)

const (
	StdioMCPBundleSchema   = "polis-controlled-stdio-mcp@1"
	StdioMCPBundleManifest = "mcp-package.json"
	maxStdioMCPBundleFiles = 64
	maxStdioMCPManifest    = 16 << 10
	maxStdioMCPArgument    = 1024
)

var ErrInvalidStdioMCPBundle = errors.New("invalid controlled stdio MCP package")

type StdioMCPBundleFileManifest struct {
	RelativePath  string `json:"relativePath"`
	MediaType     string `json:"mediaType"`
	ByteSize      int64  `json:"byteSize"`
	ContentSHA256 string `json:"contentSHA256"`
}

type StdioMCPBundleManifestData struct {
	SchemaVersion string                       `json:"schemaVersion"`
	Name          string                       `json:"name"`
	ServerName    string                       `json:"serverName"`
	ServerVersion string                       `json:"serverVersion"`
	Command       string                       `json:"command"`
	EntryPoint    string                       `json:"entryPoint"`
	Args          []string                     `json:"args"`
	Files         []StdioMCPBundleFileManifest `json:"files"`
}

type StdioMCPBundleFile struct {
	RelativePath  string
	MediaType     string
	ByteSize      int64
	ContentSHA256 string
	Content       []byte
}

type StdioMCPBundle struct {
	Manifest       StdioMCPBundleManifestData
	ManifestJSON   []byte
	ManifestDigest string
	ContentDigest  string
	Files          []StdioMCPBundleFile
}

type stdioMCPBundleDeclaration struct {
	SchemaVersion string   `json:"schemaVersion"`
	Name          string   `json:"name"`
	ServerName    string   `json:"serverName"`
	ServerVersion string   `json:"serverVersion"`
	Command       string   `json:"command"`
	EntryPoint    string   `json:"entryPoint"`
	Args          []string `json:"args"`
}

func PrepareStdioMCPBundle(archive []byte) (StdioMCPBundle, error) {
	entries, err := intake.ExtractVerifiedZIPFiles(archive)
	if err != nil || len(entries) == 0 || len(entries) > maxStdioMCPBundleFiles {
		return StdioMCPBundle{}, invalidStdioMCPBundle("invalid ZIP or file count")
	}
	var declaration stdioMCPBundleDeclaration
	declarationFound := false
	files := make([]StdioMCPBundleFile, 0, len(entries)-1)
	var totalBytes int64
	for _, entry := range entries {
		if entry.RelativePath == StdioMCPBundleManifest {
			if declarationFound || len(entry.Content) > maxStdioMCPManifest || strictMCPJSON(entry.Content, &declaration) != nil {
				return StdioMCPBundle{}, invalidStdioMCPBundle("invalid package manifest")
			}
			declarationFound = true
			continue
		}
		if !validStdioMCPRelativePath(entry.RelativePath) || len(entry.Content) == 0 {
			return StdioMCPBundle{}, invalidStdioMCPBundle("package contains an unsupported path or empty file")
		}
		copyOfContent := append([]byte(nil), entry.Content...)
		totalBytes += int64(len(copyOfContent))
		if totalBytes > intake.MaxDirectoryBytes {
			return StdioMCPBundle{}, invalidStdioMCPBundle("expanded package exceeds its byte bound")
		}
		files = append(files, StdioMCPBundleFile{
			RelativePath: entry.RelativePath, MediaType: entry.MediaType, ByteSize: int64(len(copyOfContent)),
			ContentSHA256: digestBytes(copyOfContent), Content: copyOfContent,
		})
	}
	if !declarationFound || len(files) == 0 {
		return StdioMCPBundle{}, invalidStdioMCPBundle("package manifest and executable files are required")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelativePath < files[j].RelativePath })
	manifest := StdioMCPBundleManifestData{
		SchemaVersion: declaration.SchemaVersion, Name: declaration.Name, ServerName: declaration.ServerName,
		ServerVersion: declaration.ServerVersion, Command: declaration.Command, EntryPoint: declaration.EntryPoint,
		Args: append([]string{}, declaration.Args...), Files: stdioMCPBundleFileManifest(files),
	}
	if validateStdioMCPBundleManifest(manifest) != nil || !stdioMCPBundleContains(files, manifest.Command) || !stdioMCPBundleContains(files, manifest.EntryPoint) {
		return StdioMCPBundle{}, invalidStdioMCPBundle("package metadata does not match the included files")
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil || len(manifestJSON) > intake.MaxDirectoryManifest {
		return StdioMCPBundle{}, invalidStdioMCPBundle("canonical manifest exceeds its size bound")
	}
	digest := digestBytes(manifestJSON)
	return StdioMCPBundle{Manifest: manifest, ManifestJSON: manifestJSON, ManifestDigest: digest, ContentDigest: digest, Files: files}, nil
}

func VerifyStdioMCPBundle(manifest StdioMCPBundleManifestData, manifestDigest string, files []StdioMCPBundleFile) error {
	if ValidateStdioMCPBundleManifest(manifest, manifestDigest) != nil || len(files) != len(manifest.Files) {
		return invalidStdioMCPBundle("invalid persisted manifest shape")
	}
	verified := append([]StdioMCPBundleFile(nil), files...)
	sort.Slice(verified, func(i, j int) bool { return verified[i].RelativePath < verified[j].RelativePath })
	for index, file := range verified {
		entry := manifest.Files[index]
		if !validStdioMCPRelativePath(file.RelativePath) || file.RelativePath != entry.RelativePath || file.MediaType != entry.MediaType || file.ByteSize != entry.ByteSize || file.ByteSize != int64(len(file.Content)) || file.ByteSize <= 0 || file.ContentSHA256 != entry.ContentSHA256 || file.ContentSHA256 != digestBytes(file.Content) {
			return invalidStdioMCPBundle("package file differs from its frozen manifest")
		}
	}
	if !stdioMCPBundleContains(verified, manifest.Command) || !stdioMCPBundleContains(verified, manifest.EntryPoint) {
		return invalidStdioMCPBundle("package command or entry point is missing")
	}
	return nil
}

func ValidateStdioMCPBundleManifest(manifest StdioMCPBundleManifestData, manifestDigest string) error {
	if validateStdioMCPBundleManifest(manifest) != nil {
		return invalidStdioMCPBundle("invalid persisted manifest shape")
	}
	canonical, err := json.Marshal(manifest)
	if err != nil || len(canonical) > intake.MaxDirectoryManifest || digestBytes(canonical) != manifestDigest {
		return invalidStdioMCPBundle("package manifest digest differs")
	}
	return nil
}

func validateStdioMCPBundleManifest(manifest StdioMCPBundleManifestData) error {
	if manifest.SchemaVersion != StdioMCPBundleSchema || !validStdioMCPText(manifest.Name, 160) || !validStdioMCPText(manifest.ServerName, 128) || !validStdioMCPText(manifest.ServerVersion, 128) || !validStdioMCPRelativePath(manifest.Command) || !validStdioMCPRelativePath(manifest.EntryPoint) || len(manifest.Args) > 64 || len(manifest.Files) == 0 || len(manifest.Files) > maxStdioMCPBundleFiles-1 || !sort.SliceIsSorted(manifest.Files, func(i, j int) bool { return manifest.Files[i].RelativePath < manifest.Files[j].RelativePath }) {
		return ErrInvalidStdioMCPBundle
	}
	seen := make(map[string]struct{}, len(manifest.Files))
	var totalBytes int64
	for _, file := range manifest.Files {
		key := strings.ToLower(file.RelativePath)
		if !validStdioMCPRelativePath(file.RelativePath) || file.MediaType == "" || file.ByteSize <= 0 || len(file.ContentSHA256) != 64 || !isLowerHexDigest(file.ContentSHA256) {
			return ErrInvalidStdioMCPBundle
		}
		if _, exists := seen[key]; exists {
			return ErrInvalidStdioMCPBundle
		}
		seen[key] = struct{}{}
		totalBytes += file.ByteSize
		if totalBytes > intake.MaxDirectoryBytes {
			return ErrInvalidStdioMCPBundle
		}
	}
	for _, argument := range manifest.Args {
		if !utf8.ValidString(argument) || strings.ContainsRune(argument, '\x00') || len(argument) > maxStdioMCPArgument {
			return ErrInvalidStdioMCPBundle
		}
	}
	return nil
}

func strictMCPJSON(data []byte, target any) error {
	if !utf8.Valid(data) {
		return errors.New("manifest is not valid UTF-8")
	}
	if err := rejectDuplicateMCPJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("manifest contains trailing JSON data")
	}
	return nil
}

func rejectDuplicateMCPJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var readValue func() error
	readValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			return nil
		}
		switch delimiter {
		case '{':
			keys := make(map[string]struct{})
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				key, ok := keyToken.(string)
				if keyErr != nil || !ok {
					return errors.New("manifest object key is invalid")
				}
				if _, exists := keys[key]; exists {
					return errors.New("manifest contains a duplicate object key")
				}
				keys[key] = struct{}{}
				if err = readValue(); err != nil {
					return err
				}
			}
			closing, closeErr := decoder.Token()
			if closeErr != nil || closing != json.Delim('}') {
				return errors.New("manifest object is incomplete")
			}
		case '[':
			for decoder.More() {
				if err = readValue(); err != nil {
					return err
				}
			}
			closing, closeErr := decoder.Token()
			if closeErr != nil || closing != json.Delim(']') {
				return errors.New("manifest array is incomplete")
			}
		default:
			return errors.New("manifest has an unexpected closing delimiter")
		}
		return nil
	}
	if err := readValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("manifest contains trailing JSON data")
	}
	return nil
}

func validStdioMCPRelativePath(value string) bool {
	if value == "" || len(value) > intake.MaxZIPPathBytes || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, `\:`) || strings.HasPrefix(value, "/") {
		return false
	}
	clean := path.Clean(value)
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func validStdioMCPText(value string, maxLength int) bool {
	return strings.TrimSpace(value) != "" && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && len(value) <= maxLength
}

func stdioMCPBundleFileManifest(files []StdioMCPBundleFile) []StdioMCPBundleFileManifest {
	manifest := make([]StdioMCPBundleFileManifest, 0, len(files))
	for _, file := range files {
		manifest = append(manifest, StdioMCPBundleFileManifest{RelativePath: file.RelativePath, MediaType: file.MediaType, ByteSize: file.ByteSize, ContentSHA256: file.ContentSHA256})
	}
	return manifest
}

func stdioMCPBundleContains(files []StdioMCPBundleFile, relativePath string) bool {
	for _, file := range files {
		if file.RelativePath == relativePath {
			return true
		}
	}
	return false
}

func invalidStdioMCPBundle(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidStdioMCPBundle, reason)
}

func isLowerHexDigest(value string) bool {
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
