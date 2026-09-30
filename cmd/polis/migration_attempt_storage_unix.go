//go:build !windows

// pattern: Imperative Shell
package main

import (
	"errors"
	"os"
	"path/filepath"
)

func createMigrationAttemptRoot(root string) error {
	return os.Mkdir(root, 0700)
}

func secureMigrationAttemptRoot(root string) error {
	return os.Chmod(root, 0700)
}

func syncMigrationAttemptRootParent(root string) error {
	return syncMigrationAttemptDirectory(filepath.Dir(root))
}

func persistMigrationAttemptEvent(root, path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(contents)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return syncMigrationAttemptDirectory(root)
}

func syncMigrationAttemptDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}
