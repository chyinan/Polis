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
	f, e := r.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return "", e
	}
	defer r.Remove(stage)
	if _, e = f.Write(content); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if e = r.Chmod(stage, 0444); e != nil {
		return "", e
	}
	if e = r.Rename(stage, digest); e != nil {
		return "", e
	}
	dir, e := r.Open(".")
	if e != nil {
		return "", e
	}
	e = dir.Sync()
	closeErr = dir.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	return digest, nil
}
func readBlob(root, company, digest string) ([]byte, error) {
	if len(digest) != 64 {
		return nil, core.Integrity
	}
	if _, e := hex.DecodeString(digest); e != nil {
		return nil, core.Integrity
	}
	r, e := blobDir(root, company)
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
	if !info.Mode().IsRegular() {
		return nil, core.Integrity
	}
	b, e := io.ReadAll(io.LimitReader(f, core.MaxContent+1))
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(b)
	if len(b) > core.MaxContent || hex.EncodeToString(h[:]) != digest {
		return nil, core.Integrity
	}
	return b, nil
}
