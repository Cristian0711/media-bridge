//go:build unix

package health

import (
	"os"
	"syscall"
)

func fileNLink(info os.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	// st.Nlink is uint64 on Linux but uint16 on darwin; keep the conversion.
	return uint64(st.Nlink), true //nolint:unconvert
}
