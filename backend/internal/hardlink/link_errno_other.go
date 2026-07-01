//go:build !unix

package hardlink

func isCrossDeviceLinkErrno(err error) bool {
	return false
}
