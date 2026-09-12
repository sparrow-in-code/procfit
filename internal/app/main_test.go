package app

import (
	"os"
	"testing"
)

// TestMain isolates app tests from any real user config: config discovery keys
// off XDG_CONFIG_HOME, so pointing it at an empty temp dir guarantees no stray
// config file influences command behaviour during tests.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "procfit-test-cfg")
	if err == nil {
		os.Setenv("XDG_CONFIG_HOME", dir)
	}
	code := m.Run()
	if dir != "" {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}
