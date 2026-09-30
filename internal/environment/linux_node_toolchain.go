// pattern: Imperative Shell
package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	maxLinuxNodeSystemImageEntries = 250_000
	maxLinuxNodeSystemImageBytes   = int64(2 << 30)
	maxLinuxNodeToolExecutableSize = int64(256 << 20)
)

var errLinuxNodeToolchain = errors.New("invalid Linux Node/npm sandbox toolchain")

type linuxNodeToolchainManifest struct {
	SchemaVersion           string                  `json:"schemaVersion"`
	BubblewrapSHA256        string                  `json:"bubblewrapSha256"`
	SystemImageSHA256       string                  `json:"systemImageSha256"`
	NPMCacheSHA256          string                  `json:"npmCacheSha256"`
	CgroupRoot              string                  `json:"cgroupRoot"`
	ResourceLimits          LinuxNodeResourceLimits `json:"resourceLimits"`
	WorkspaceDiskLimitBytes uint64                  `json:"workspaceDiskLimitBytes"`
	NodeExecutable          string                  `json:"nodeExecutable"`
	NPMCLIScript            string                  `json:"npmCliScript"`
}

type linuxNodeSystemImageEntry struct {
	path   string
	rel    string
	info   os.FileInfo
	mode   string
	owner  string
	kind   string
	digest string
	target string
}

func LinuxNodeToolchainSHA256(paths LinuxNodeSandboxPaths) (string, error) {
	if err := ValidateLinuxNodeSandboxPaths(paths); err != nil {
		return "", errLinuxNodeToolchain
	}
	nodePath := filepath.Join(paths.SystemImageRoot, filepath.FromSlash(strings.TrimPrefix(paths.NodeExecutable, "/")))
	npmPath := filepath.Join(paths.SystemImageRoot, filepath.FromSlash(strings.TrimPrefix(paths.NPMCLIScript, "/")))
	if !linuxRegularExecutable(nodePath) || !linuxRegularContainedPath(paths.SystemImageRoot, npmPath) {
		return "", errLinuxNodeToolchain
	}
	bwrapContentDigest, bwrapMode, bwrapOwner, bwrapOwnerKnown, err := hashLinuxRegularToolIdentity(filepath.Clean(paths.BubblewrapPath), maxLinuxNodeToolExecutableSize)
	if err != nil || !bwrapOwnerKnown {
		return "", errLinuxNodeToolchain
	}
	bwrapIdentity := sha256.New()
	writeLinuxToolchainField(bwrapIdentity, []byte(bwrapContentDigest))
	writeLinuxToolchainField(bwrapIdentity, []byte(bwrapMode))
	writeLinuxToolchainField(bwrapIdentity, []byte(bwrapOwner))
	bwrapDigest := hex.EncodeToString(bwrapIdentity.Sum(nil))
	imageDigest, err := LinuxNodeSystemImageSHA256(paths.SystemImageRoot)
	if err != nil {
		return "", err
	}
	cacheDigest, err := LinuxNodeNPMCacheSHA256(paths.NPMCacheRoot)
	if err != nil {
		return "", err
	}
	manifest := linuxNodeToolchainManifest{
		SchemaVersion: "linux-node-toolchain@5", BubblewrapSHA256: bwrapDigest,
		SystemImageSHA256: imageDigest, NPMCacheSHA256: cacheDigest,
		CgroupRoot: filepath.Clean(paths.CgroupRoot), ResourceLimits: DefaultLinuxNodeResourceLimits(),
		WorkspaceDiskLimitBytes: EffectiveLinuxNodeWorkspaceDiskLimitBytes(paths),
		NodeExecutable:          paths.NodeExecutable, NPMCLIScript: paths.NPMCLIScript,
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return "", errLinuxNodeToolchain
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func LinuxNodeSystemImageSHA256(root string) (string, error) {
	return linuxNodeSystemImageSHA256WithObserver(root, nil)
}

func linuxNodeSystemImageSHA256WithObserver(root string, afterEntry func(string) error) (string, error) {
	if !linuxSandboxRoot(root) {
		return "", errLinuxNodeToolchain
	}
	root = filepath.Clean(root)
	h := sha256.New()
	count := 0
	totalBytes := int64(0)
	entries := make([]linuxNodeSystemImageEntry, 0, 1024)
	writeField := func(value []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(value)
	}
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errLinuxNodeToolchain
		}
		relative, err := filepath.Rel(root, current)
		if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errLinuxNodeToolchain
		}
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSocket != 0 || info.Mode()&os.ModeNamedPipe != 0 || info.Mode()&os.ModeDevice != 0 {
			return errLinuxNodeToolchain
		}
		count++
		if count > maxLinuxNodeSystemImageEntries {
			return errLinuxNodeToolchain
		}
		if len(relative) > 4096 {
			return errLinuxNodeToolchain
		}
		writeField([]byte(filepath.ToSlash(relative)))
		ownerFingerprint, ownerKnown := linuxNodeOwnershipFingerprint(info.Sys())
		if !ownerKnown {
			return errLinuxNodeToolchain
		}
		writeField([]byte(info.Mode().String()))
		writeField([]byte(ownerFingerprint))
		imageEntry := linuxNodeSystemImageEntry{
			path: current, rel: filepath.ToSlash(relative), info: info,
			mode: info.Mode().String(), owner: ownerFingerprint,
		}
		switch {
		case info.Mode().IsDir():
			imageEntry.kind = "directory"
			writeField([]byte("directory"))
		case info.Mode().IsRegular():
			if info.Size() < 0 || info.Size() > maxLinuxNodeSystemImageBytes-totalBytes {
				return errLinuxNodeToolchain
			}
			contentDigest, fileMode, fileOwner, fileOwnerKnown, err := hashLinuxRegularToolIdentity(current, info.Size())
			if err != nil || !fileOwnerKnown || fileMode != info.Mode().String() || fileOwner != ownerFingerprint {
				return errLinuxNodeToolchain
			}
			totalBytes += info.Size()
			imageEntry.kind, imageEntry.digest = "file", contentDigest
			writeField([]byte(contentDigest))
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(current)
			if err != nil || len(target) > 4096 || strings.ContainsRune(target, '\x00') {
				return errLinuxNodeToolchain
			}
			imageEntry.kind, imageEntry.target = "symlink", target
			writeField([]byte("symlink"))
			writeField([]byte(target))
		default:
			return errLinuxNodeToolchain
		}
		entries = append(entries, imageEntry)
		if afterEntry != nil {
			if err = afterEntry(current); err != nil {
				return errLinuxNodeToolchain
			}
		}
		return nil
	})
	if err != nil {
		return "", errLinuxNodeToolchain
	}
	if !linuxNodeSystemImagePathSetMatches(root, entries) {
		return "", errLinuxNodeToolchain
	}
	for _, entry := range entries {
		if !linuxNodeSystemImageEntryStable(entry) {
			return "", errLinuxNodeToolchain
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func linuxNodeSystemImagePathSetMatches(root string, entries []linuxNodeSystemImageEntry) bool {
	expected := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		expected[entry.rel] = struct{}{}
	}
	seen := 0
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errLinuxNodeToolchain
		}
		relative, relErr := filepath.Rel(root, current)
		if relErr != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errLinuxNodeToolchain
		}
		relative = filepath.ToSlash(relative)
		if _, ok := expected[relative]; !ok {
			return errLinuxNodeToolchain
		}
		delete(expected, relative)
		seen++
		return nil
	})
	return err == nil && seen == len(entries) && len(expected) == 0
}

func linuxNodeSystemImageEntryStable(entry linuxNodeSystemImageEntry) bool {
	current, err := os.Lstat(entry.path)
	if err != nil || !linuxNodeSystemImageInfoMatches(entry, current) {
		return false
	}
	switch entry.kind {
	case "directory":
		return true
	case "file":
		digest, mode, owner, ownerKnown, hashErr := hashLinuxRegularToolIdentity(entry.path, entry.info.Size())
		if hashErr != nil || !ownerKnown || digest != entry.digest || mode != entry.mode || owner != entry.owner {
			return false
		}
		verified, statErr := os.Lstat(entry.path)
		return statErr == nil && linuxNodeSystemImageInfoMatches(entry, verified)
	case "symlink":
		target, readErr := os.Readlink(entry.path)
		verified, statErr := os.Lstat(entry.path)
		return readErr == nil && statErr == nil && target == entry.target && linuxNodeSystemImageInfoMatches(entry, verified)
	default:
		return false
	}
}

func linuxNodeSystemImageInfoMatches(entry linuxNodeSystemImageEntry, current os.FileInfo) bool {
	if !os.SameFile(entry.info, current) || current.Mode().String() != entry.mode || !current.ModTime().Equal(entry.info.ModTime()) {
		return false
	}
	owner, ownerKnown := linuxNodeOwnershipFingerprint(current.Sys())
	return ownerKnown && owner == entry.owner
}

func linuxNodeOwnershipFingerprint(stat any) (string, bool) {
	value := reflect.ValueOf(stat)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return "", false
	}
	uid, uidOK := linuxNodeUnsignedStatField(value, "Uid")
	gid, gidOK := linuxNodeUnsignedStatField(value, "Gid")
	if !uidOK || !gidOK {
		return "", false
	}
	return fmt.Sprintf("uid=%d;gid=%d", uid, gid), true
}

func linuxNodeUnsignedStatField(stat reflect.Value, name string) (uint64, bool) {
	field := stat.FieldByName(name)
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint(), true
	default:
		return 0, false
	}
}

func hashLinuxRegularToolIdentity(path string, limit int64) (string, string, string, bool, error) {
	if !filepath.IsAbs(path) || limit <= 0 || !linuxNoLinkAncestors(path) {
		return "", "", "", false, errLinuxNodeToolchain
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > limit {
		return "", "", "", false, errLinuxNodeToolchain
	}
	beforeOwner, beforeOwnerKnown := linuxNodeOwnershipFingerprint(before.Sys())
	file, err := os.Open(path)
	if err != nil {
		return "", "", "", false, errLinuxNodeToolchain
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", "", "", false, errLinuxNodeToolchain
	}
	openedOwner, openedOwnerKnown := linuxNodeOwnershipFingerprint(opened.Sys())
	if !os.SameFile(before, opened) || before.Mode() != opened.Mode() || beforeOwner != openedOwner || beforeOwnerKnown != openedOwnerKnown {
		return "", "", "", false, errLinuxNodeToolchain
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, limit+1))
	if err != nil || count != before.Size() || count > limit {
		return "", "", "", false, errLinuxNodeToolchain
	}
	after, err := file.Stat()
	if err != nil {
		return "", "", "", false, errLinuxNodeToolchain
	}
	current, currentErr := os.Lstat(path)
	if currentErr != nil {
		return "", "", "", false, errLinuxNodeToolchain
	}
	afterOwner, afterOwnerKnown := linuxNodeOwnershipFingerprint(after.Sys())
	currentOwner, currentOwnerKnown := linuxNodeOwnershipFingerprint(current.Sys())
	if !os.SameFile(before, after) || !os.SameFile(before, current) || before.Size() != after.Size() || before.ModTime() != after.ModTime() || current.ModTime() != before.ModTime() || before.Mode() != after.Mode() || before.Mode() != current.Mode() || beforeOwner != afterOwner || beforeOwner != currentOwner || beforeOwnerKnown != afterOwnerKnown || beforeOwnerKnown != currentOwnerKnown || current.Mode()&os.ModeSymlink != 0 {
		return "", "", "", false, errLinuxNodeToolchain
	}
	return hex.EncodeToString(digest.Sum(nil)), before.Mode().String(), beforeOwner, beforeOwnerKnown, nil
}

func writeLinuxToolchainField(writer io.Writer, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}

func validateLinuxNodeToolchainSHA256(value string) error {
	if len(value) != 64 {
		return fmt.Errorf("%w: invalid digest", errLinuxNodeToolchain)
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return fmt.Errorf("%w: invalid digest", errLinuxNodeToolchain)
		}
	}
	return nil
}
