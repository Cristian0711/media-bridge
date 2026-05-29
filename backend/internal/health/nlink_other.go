//go:build !unix

package health

import "os"

func fileNLink(info os.FileInfo) (uint64, bool) {
	// Hardlink count audit is only meaningful on Unix deployments (Docker/Linux).
	return 0, false
}
