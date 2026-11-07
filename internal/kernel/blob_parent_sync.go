// pattern: Imperative Shell

package kernel

import (
	"errors"
	"fmt"
	"os"
)

var ErrParentDirectorySyncUnsupported = errors.New("parent directory sync unsupported")

func syncParentDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		if isParentDirectorySyncUnsupported(err) {
			return fmt.Errorf("%w: %v", ErrParentDirectorySyncUnsupported, err)
		}
		return err
	}
	return closeErr
}
