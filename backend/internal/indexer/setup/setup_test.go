package setup_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/config"
	"github.com/Cristian0711/media-bridge/backend/internal/indexer/setup"
)

// TestNewServiceWiresProwlarrProvider is a smoke test: NewService must assemble
// an indexer.Service with the Prowlarr provider registered (both as an indexer
// and as the browse catalog) without panicking, for enabled and disabled config.
func TestNewServiceWiresProwlarrProvider(t *testing.T) {
	t.Parallel()
	for _, enabled := range []bool{true, false} {
		cfg := config.IndexerConfig{
			Prowlarr: config.ProwlarrConfig{
				Enabled: enabled,
				URL:     "http://127.0.0.1:9696",
				APIKey:  "key",
			},
		}
		if svc := setup.NewService(cfg); svc == nil {
			t.Fatalf("NewService(enabled=%v) returned nil", enabled)
		}
	}
}
