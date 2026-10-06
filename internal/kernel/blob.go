// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"polis/internal/core"
)

const maxBoundedCASReadBytes int64 = 8 << 20

func blobDir(root, company string) (*os.Root, error) {
	if !core.ValidID(company) {
		return nil, core.Malformed
	}
	p := filepath.Join(root, company)
	if e := os.MkdirAll(p, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, core.Denied
	}
	return os.OpenRoot(p)
}

func blobDirReadOnly(root, company string) (*os.Root, error) {
	if !core.ValidID(company) {
		return nil, core.Malformed
	}
	p := filepath.Join(root, company)
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, core.Denied
	}
	return os.OpenRoot(p)
}
func putBlob(root, company string, content []byte) (string, error) {
	h := sha256.Sum256(content)
	digest := hex.EncodeToString(h[:])
	r, e := blobDir(root, company)
	if e != nil {
		return "", e
	}
	defer r.Close()
	if old, e := r.ReadFile(digest); e == nil {
		if !bytes.Equal(old, content) {
			return "", core.Integrity
		}
		return digest, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	stage := ".stage-" + newID()
	if e := finalizeBlob(r, stage, digest, content, defaultBlobFinalizeOperations()); e != nil {
		return "", e
	}
	return digest, nil
}
func readBlob(root, company, digest string) ([]byte, error) {
	return readBlobBounded(root, company, digest, core.MaxContent)
}

func readBlobBounded(root, company, digest string, maxBytes int64) ([]byte, error) {
	if maxBytes < 1 || maxBytes > maxBoundedCASReadBytes {
		return nil, core.Malformed
	}
	if len(digest) != 64 {
		return nil, core.Integrity
	}
	if _, e := hex.DecodeString(digest); e != nil {
		return nil, core.Integrity
	}
	r, e := blobDirReadOnly(root, company)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	f, e := r.Open(digest)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxBytes {
		return nil, core.Integrity
	}
	b, e := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(b)
	if int64(len(b)) > maxBytes || hex.EncodeToString(h[:]) != digest {
		return nil, core.Integrity
	}
	return b, nil
}
