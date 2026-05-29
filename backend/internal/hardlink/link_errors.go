package hardlink

import (
	"fmt"

	processingqueue "github.com/Cristian0711/media-bridge/backend/shared/processing-queue"
)

func crossDeviceLinkError(sourcePath, destPath string, linkErr error) error {
	return fmt.Errorf(
		"hardlink failed: download and library must be on the same filesystem (cross-device link from %s to %s): %v: %w",
		sourcePath, destPath, linkErr, processingqueue.ErrPermanentFailure,
	)
}
