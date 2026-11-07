// pattern: Imperative Shell

package kernel

import (
	"errors"
	"os"
)

type blobFinalizeOperations struct {
	syncFile   func(*os.File) error
	rename     func(*os.Root, string, string) error
	syncParent func(*os.Root) error
}

func defaultBlobFinalizeOperations() blobFinalizeOperations {
	return blobFinalizeOperations{
		syncFile: func(file *os.File) error { return file.Sync() },
		rename: func(root *os.Root, from, to string) error {
			return root.Rename(from, to)
		},
		syncParent: syncParentDirectory,
	}
}

func finalizeBlob(root *os.Root, stage, digest string, content []byte, operations blobFinalizeOperations) error {
	if operations.syncFile == nil || operations.rename == nil || operations.syncParent == nil {
		return errors.New("blob finalization operations are incomplete")
	}
	file, err := root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(stage)
	if _, err = file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err = operations.syncFile(file); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = root.Chmod(stage, 0444); err != nil {
		return err
	}
	if err = operations.rename(root, stage, digest); err != nil {
		return err
	}
	if err = operations.syncParent(root); err != nil {
		if !parentDirectorySyncUnsupportedAllowed() || !errors.Is(err, ErrParentDirectorySyncUnsupported) {
			return err
		}
	}
	return nil
}
