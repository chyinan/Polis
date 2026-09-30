// pattern: Imperative Shell
package environment

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type linuxNodeCacheEntry struct {
	relative  string
	digest    string
	directory bool
}

// LinuxNodeNPMCacheSHA256 binds an offline npm cache to the selected toolchain.
// The cache is input data: links and special files are rejected and traversal
// is bounded before any workspace is prepared.
func LinuxNodeNPMCacheSHA256(root string) (string, error) {
	entries, digest, err := inspectLinuxNodeNPMCache(root)
	_ = entries
	return digest, err
}

// MaterializeLinuxNodeNPMCache copies a qualified, bounded cache into a newly
// created empty per-workspace directory. The caller never exposes the shared
// cache to project code.
func MaterializeLinuxNodeNPMCache(sourceRoot, destinationRoot string) error {
	sourceEntries, sourceDigest, err := inspectLinuxNodeNPMCache(sourceRoot)
	if err != nil {
		return err
	}
	destinationInfo, err := os.Lstat(destinationRoot)
	if err != nil || !destinationInfo.IsDir() || destinationInfo.Mode()&os.ModeSymlink != 0 {
		return errLinuxNodeToolchain
	}
	items, err := os.ReadDir(destinationRoot)
	if err != nil || len(items) != 0 {
		return errLinuxNodeToolchain
	}
	for _, entry := range sourceEntries {
		if entry.relative == "." {
			continue
		}
		source := filepath.Join(sourceRoot, filepath.FromSlash(entry.relative))
		destination := filepath.Join(destinationRoot, filepath.FromSlash(entry.relative))
		if !linuxDirectoryContainedPath(sourceRoot, filepath.Dir(source)) {
			return errLinuxNodeToolchain
		}
		if entry.directory {
			if err = os.Mkdir(destination, 0700); err != nil {
				return errLinuxNodeToolchain
			}
			continue
		}
		if err = copyLinuxCacheFile(source, destination, entry.digest); err != nil {
			return err
		}
	}
	_, copiedDigest, copiedErr := inspectLinuxNodeNPMCache(destinationRoot)
	_, currentSourceDigest, sourceErr := inspectLinuxNodeNPMCache(sourceRoot)
	if copiedErr != nil || sourceErr != nil || copiedDigest != sourceDigest || currentSourceDigest != sourceDigest {
		return errLinuxNodeToolchain
	}
	return nil
}

func inspectLinuxNodeNPMCache(root string) ([]linuxNodeCacheEntry, string, error) {
	if !linuxSandboxRoot(root) {
		return nil, "", errLinuxNodeToolchain
	}
	root = filepath.Clean(root)
	entries := make([]linuxNodeCacheEntry, 0, 128)
	count := 0
	totalBytes := int64(0)
	err := filepath.WalkDir(root, func(current string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errLinuxNodeToolchain
		}
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return errLinuxNodeToolchain
		}
		relative, relErr := filepath.Rel(root, current)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || len(relative) > 4096 {
			return errLinuxNodeToolchain
		}
		count++
		if count > maxLinuxNodeSystemImageEntries {
			return errLinuxNodeToolchain
		}
		entry := linuxNodeCacheEntry{relative: filepath.ToSlash(relative), directory: info.IsDir()}
		if !info.IsDir() {
			if info.Size() < 0 || info.Size() > maxLinuxNodeSystemImageBytes-totalBytes {
				return errLinuxNodeToolchain
			}
			entry.digest, statErr = hashLinuxCacheFile(current, info)
			if statErr != nil {
				return errLinuxNodeToolchain
			}
			totalBytes += info.Size()
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, "", errLinuxNodeToolchain
	}
	hash := sha256.New()
	for _, entry := range entries {
		writeLinuxCacheField(hash, []byte(entry.relative))
		if entry.directory {
			writeLinuxCacheField(hash, []byte("directory"))
		} else {
			writeLinuxCacheField(hash, []byte("file"))
			writeLinuxCacheField(hash, []byte(entry.digest))
		}
	}
	return entries, hex.EncodeToString(hash.Sum(nil)), nil
}

func hashLinuxCacheFile(path string, before os.FileInfo) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errLinuxNodeToolchain
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", errLinuxNodeToolchain
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, before.Size()+1))
	if err != nil || count != before.Size() {
		return "", errLinuxNodeToolchain
	}
	after, err := file.Stat()
	current, lstatErr := os.Lstat(path)
	if err != nil || lstatErr != nil || !os.SameFile(before, after) || !os.SameFile(before, current) || after.Size() != before.Size() || after.ModTime() != before.ModTime() || current.Mode()&os.ModeSymlink != 0 {
		return "", errLinuxNodeToolchain
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func copyLinuxCacheFile(source, destination, expectedDigest string) error {
	before, err := os.Lstat(source)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > maxLinuxNodeSystemImageBytes {
		return errLinuxNodeToolchain
	}
	input, err := os.Open(source)
	if err != nil {
		return errLinuxNodeToolchain
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errLinuxNodeToolchain
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errLinuxNodeToolchain
	}
	digest := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(output, digest), io.LimitReader(input, before.Size()+1))
	syncErr := output.Sync()
	closeErr := output.Close()
	after, statErr := input.Stat()
	current, currentErr := os.Lstat(source)
	if copyErr != nil || syncErr != nil || closeErr != nil || statErr != nil || currentErr != nil || count != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, current) || after.Size() != before.Size() || after.ModTime() != before.ModTime() || current.Mode()&os.ModeSymlink != 0 || hex.EncodeToString(digest.Sum(nil)) != expectedDigest {
		_ = os.Remove(destination)
		return errors.Join(errLinuxNodeToolchain, copyErr, syncErr, closeErr, statErr, currentErr)
	}
	return nil
}

func writeLinuxCacheField(writer io.Writer, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}
