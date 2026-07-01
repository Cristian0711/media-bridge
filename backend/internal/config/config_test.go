package config

import (
	"os"
	"testing"
)

// setRequired sets the three env vars Load treats as mandatory so a test can
// focus on the field it cares about. Uses t.Setenv, which restores on cleanup.
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("TMDB_API_KEY", "tmdb-key")
}

func TestLoadDefaults(t *testing.T) {
	// Clear the optional vars so we exercise the fallback branch of get/getBool/getInt.
	for _, k := range []string{
		"PORT", "MOVIES_PATH", "SHOWS_PATH", "DOWNLOADS_PATH",
		"REQUESTS_QUEUE_WORKERS", "DOWNLOAD_QUEUE_WORKERS", "HARDLINK_QUEUE_WORKERS", "REMOVE_QUEUE_WORKERS",
		"QBITTORRENT_URL", "QBITTORRENT_USERNAME", "QBITTORRENT_PASSWORD",
		"PROWLARR_ENABLED", "PROWLARR_URL", "PROWLARR_API_KEY", "TMDB_URL",
	} {
		t.Setenv(k, "")
	}
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
	if cfg.QueueWorkers.Download != 2 {
		t.Errorf("Download workers = %d, want default 2", cfg.QueueWorkers.Download)
	}
	if !cfg.Indexer.Prowlarr.Enabled {
		t.Error("Prowlarr.Enabled = false, want default true")
	}
	if cfg.QBittorrent.Password != defaultQBittorrentPassword {
		t.Errorf("QBittorrent.Password = %q, want default", cfg.QBittorrent.Password)
	}
}

func TestLoadMissingRequiredVars(t *testing.T) {
	cases := []struct{ clear string }{
		{"DATABASE_URL"},
		{"JWT_SECRET"},
		{"TMDB_API_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.clear, func(t *testing.T) {
			setRequired(t)
			t.Setenv(tc.clear, "")
			if _, err := Load(); err == nil {
				t.Fatalf("Load() with %s unset: expected error, got nil", tc.clear)
			}
		})
	}
}

func TestLoadOverridesFromEnv(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "9999")
	t.Setenv("PROWLARR_ENABLED", "false")
	t.Setenv("DOWNLOAD_QUEUE_WORKERS", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("Port = %q, want 9999", cfg.Port)
	}
	if cfg.Indexer.Prowlarr.Enabled {
		t.Error("Prowlarr.Enabled = true, want false from env")
	}
	if cfg.QueueWorkers.Download != 5 {
		t.Errorf("Download workers = %d, want 5", cfg.QueueWorkers.Download)
	}
}

func TestGet(t *testing.T) {
	t.Setenv("SOME_KEY", "value")
	if got := get("SOME_KEY", "fallback"); got != "value" {
		t.Errorf("get(set) = %q, want value", got)
	}
	if got := get("UNSET_KEY_XYZ", "fallback"); got != "fallback" {
		t.Errorf("get(unset) = %q, want fallback", got)
	}
}

func TestGetBool(t *testing.T) {
	cases := []struct {
		val  string
		set  bool
		want bool
	}{
		{"", false, true},       // unset → fallback (true)
		{"true", true, true},    //
		{"1", true, true},       //
		{"yes", true, true},     //
		{"false", true, false},  //
		{"0", true, false},      //
		{"no", true, false},     //
		{"FALSE", true, false},  // case-insensitive
		{"garbage", true, true}, // any other value is truthy
	}
	for _, tc := range cases {
		name := tc.val
		if !tc.set {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			os.Unsetenv("BOOL_KEY")
			if tc.set {
				t.Setenv("BOOL_KEY", tc.val)
			}
			if got := getBool("BOOL_KEY", true); got != tc.want {
				t.Errorf("getBool(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	cases := []struct {
		val      string
		set      bool
		want     int
		fallback int
	}{
		{"", false, 3, 3},   // unset → fallback
		{"7", true, 7, 3},   // valid
		{"0", true, 3, 3},   // < 1 → fallback
		{"-4", true, 3, 3},  // negative → fallback
		{"x", true, 3, 3},   // non-numeric → fallback
		{" 5 ", true, 5, 3}, // trimmed
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			os.Unsetenv("INT_KEY")
			if tc.set {
				t.Setenv("INT_KEY", tc.val)
			}
			if got := getInt("INT_KEY", tc.fallback); got != tc.want {
				t.Errorf("getInt(%q) = %d, want %d", tc.val, got, tc.want)
			}
		})
	}
}
