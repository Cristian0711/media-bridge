//go:build unix

package remove

import (
	"os"
	"syscall"
)

// keyOf returns the (device, inode) of a regular file, the pair the Linux
// kernel uses to identify the underlying inode that every hardlink shares.
// Two files have equal keyOf() iff they are hardlinks to the same inode.
//
// Unix-only — Stat_t isn't available on Windows. The project deploys via
// Docker (Linux) so that's fine; the build tag stops it leaking into a
// non-Unix build.
func keyOf(info os.FileInfo) (fileKey, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileKey{}, false
	}
	return fileKey{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
}
