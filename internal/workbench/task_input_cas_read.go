// pattern: Imperative Shell
package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"polis/internal/core"
	"polis/internal/intake"
)

func (s *PostgresReadStore) readTaskInputBlob(companyID, digest string, expectedBytes int64) ([]byte, error) {
	if s.blobRoot == "" || expectedBytes <= 0 || expectedBytes > intake.MaxUploadBytes || len(digest) != 64 {
		return nil, core.Integrity
	}
	decodedDigest, err := hex.DecodeString(digest)
	if err != nil || hex.EncodeToString(decodedDigest) != digest {
		return nil, core.Integrity
	}
	root := filepath.Clean(s.blobRoot)
	path := filepath.Join(root, companyID, digest)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, core.Integrity
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != expectedBytes {
		return nil, core.Integrity
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || openedInfo.Size() != expectedBytes {
		return nil, core.Integrity
	}
	content, err := io.ReadAll(io.LimitReader(file, expectedBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) != expectedBytes {
		return nil, core.Integrity
	}
	contentDigest := sha256.Sum256(content)
	if hex.EncodeToString(contentDigest[:]) != digest {
		return nil, errors.New("Task input archive CAS digest mismatch")
	}
	return content, nil
}
