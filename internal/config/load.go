package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/netikras/procfit/internal/meta"
)

// Load decodes, normalizes, and validates configuration bytes in a format.
func Load(data []byte, format Format, v Validator) (Config, error) {
	raw, err := decodeRaw(data, format)
	if err != nil {
		return Config{}, err
	}
	c, err := normalize(raw)
	if err != nil {
		return Config{}, err
	}
	if err := v.Validate(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// LoadFile loads configuration from a file, choosing the decoder by extension.
func LoadFile(path string, v Validator) (Config, error) {
	format, ok := FormatFromExt(filepath.Ext(path))
	if !ok {
		return Config{}, fmt.Errorf("config: unrecognized extension for %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Load(data, format, v)
}

// Discovery describes how to locate the config file (RFC §9.2).
type Discovery struct {
	NoConfig  bool
	Explicit  string // --config
	EnvValue  string // $PROCFIT_CONFIG
	ConfigDir string // resolved XDG config dir base (…/procfit)
}

// Discover returns the config file path to load, or "" if none applies. An
// absent default is not an error; multiple default files is an error (RFC §9.2).
func (d Discovery) Discover() (string, error) {
	if d.NoConfig {
		return "", nil
	}
	if d.Explicit != "" {
		return d.Explicit, nil
	}
	if d.EnvValue != "" {
		return d.EnvValue, nil
	}
	if d.ConfigDir == "" {
		return "", nil
	}
	candidates := []string{
		filepath.Join(d.ConfigDir, "config.toml"),
		filepath.Join(d.ConfigDir, "config.yaml"),
		filepath.Join(d.ConfigDir, "config.yml"),
		filepath.Join(d.ConfigDir, "config.json"),
	}
	var found []string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			found = append(found, c)
		}
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("config: multiple default config files found, remove all but one: %v", found)
	}
}

// DefaultConfigDir returns the XDG config directory for procfit.
func DefaultConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, meta.Name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", meta.Name)
}
