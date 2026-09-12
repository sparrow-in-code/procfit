package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatFromExt(t *testing.T) {
	cases := map[string]Format{".toml": FormatTOML, ".yaml": FormatYAML, ".yml": FormatYAML, ".json": FormatJSON}
	for ext, want := range cases {
		got, ok := FormatFromExt(ext)
		if !ok || got != want {
			t.Errorf("FormatFromExt(%q) = %v,%v want %v", ext, got, ok, want)
		}
	}
	if _, ok := FormatFromExt(".ini"); ok {
		t.Error(".ini should be unknown")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	d := Discovery{ConfigDir: dir}
	if p, err := d.Discover(); err != nil || p != "" {
		t.Fatalf("empty dir should discover nothing: %q %v", p, err)
	}
	// Single file.
	toml := filepath.Join(dir, "config.toml")
	os.WriteFile(toml, []byte("version = 1\n"), 0o644)
	if p, err := d.Discover(); err != nil || p != toml {
		t.Fatalf("single file discover: %q %v", p, err)
	}
	// Multiple defaults -> error.
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644)
	if _, err := d.Discover(); err == nil {
		t.Fatal("multiple default files should error")
	}
	// Explicit and NoConfig precedence.
	if p, _ := (Discovery{Explicit: "/x.toml", ConfigDir: dir}).Discover(); p != "/x.toml" {
		t.Fatal("explicit should win")
	}
	if p, _ := (Discovery{NoConfig: true, Explicit: "/x.toml"}).Discover(); p != "" {
		t.Fatal("no-config should load nothing")
	}
	if p, _ := (Discovery{EnvValue: "/env.toml"}).Discover(); p != "/env.toml" {
		t.Fatal("env value should be used")
	}
}

func TestLoadFileUnknownExt(t *testing.T) {
	if _, err := LoadFile("/tmp/x.ini", Validator{}); err == nil {
		t.Fatal("unknown extension should error")
	}
}

func TestEncodeUnsupported(t *testing.T) {
	if _, err := Encode(Config{Version: 1}, Format("xml")); err == nil {
		t.Fatal("unsupported encode format should error")
	}
}
