//go:build windows

// pattern: Imperative Shell
package main

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func createMigrationAttemptRoot(root string) error {
	temporary, err := os.MkdirTemp(filepath.Dir(root), ".polis-migration-root-")
	if err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	to, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	if err = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}

func secureMigrationAttemptRoot(string) error {
	return nil
}

func syncMigrationAttemptRootParent(string) error {
	return nil
}

func persistMigrationAttemptEvent(root, path string, contents []byte) error {
	file, err := os.CreateTemp(root, ".polis-migration-event-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	_, writeErr := file.Write(contents)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	if err = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}
