//go:build unix

package hardlink

import (
	"errors"
	"syscall"
)

func isCrossDeviceLinkErrno(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
