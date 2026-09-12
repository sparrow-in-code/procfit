package config

import (
	"reflect"
	"testing"

	"github.com/netikras/procfit/internal/metrics"
)

// testValidator builds a permissive-but-real validator from the metric registry.
func testValidator() Validator {
	reg := metrics.NewDefault()
	dims := map[string]bool{"app": true, "session": true, "comm": true, "pidns": true}
	entity := map[string]bool{"app": true, "comm": true, "uid": true, "cmdline": true}
	for _, d := range reg.All() {
		entity[string(d.ID)] = true
	}
	return Validator{
		HasMetric:    func(s string) bool { return reg.Has(s) },
		HasDimension: func(s string) bool { return dims[s] },
		IsRateField: func(s string) bool {
			d, ok := reg.Get(s)
			return ok && d.IsRate()
		},
		EntityFields: entity,
		KnownProfile: metrics.IsKnownProfile,
	}
}

const tomlCfg = `
version = 1
interval = "1s"
group_by = ["app", "session"]
leaf = "process"
columns = ["target", "pt", "cpu", "rss"]

[intervals]
process = "1s"
fd = "5s"

[metrics]
profile = "light"
enable = ["disk-rbps"]
disable = ["vsz"]

[[sort]]
field = "cpu"
direction = "desc"

[state]
history = false

[daemon]
use = "auto"

[presets.battery]
interval = "2s"
group_by = ["app"]
leaf = "process"

[presets.battery.metrics]
profile = "light"

[[managed]]
name = "Browsers"
selector = 'app in ["google-chrome", "firefox"]'

[managed.control]
nice = 10
`

const yamlCfg = `
version: 1
interval: 1s
group_by: [app, session]
leaf: process
columns: [target, pt, cpu, rss]
intervals:
  process: 1s
  fd: 5s
metrics:
  profile: light
  enable: [disk-rbps]
  disable: [vsz]
sort:
  - field: cpu
    direction: desc
state:
  history: false
daemon:
  use: auto
presets:
  battery:
    interval: 2s
    group_by: [app]
    leaf: process
    metrics:
      profile: light
managed:
  - name: Browsers
    selector: 'app in ["google-chrome", "firefox"]'
    control:
      nice: 10
`

const jsonCfg = `
{
  "version": 1,
  "interval": "1s",
  "group_by": ["app", "session"],
  "leaf": "process",
  "columns": ["target", "pt", "cpu", "rss"],
  "intervals": {"process": "1s", "fd": "5s"},
  "metrics": {"profile": "light", "enable": ["disk-rbps"], "disable": ["vsz"]},
  "sort": [{"field": "cpu", "direction": "desc"}],
  "state": {"history": false},
  "daemon": {"use": "auto"},
  "presets": {"battery": {"interval": "2s", "group_by": ["app"], "leaf": "process", "metrics": {"profile": "light"}}},
  "managed": [{"name": "Browsers", "selector": "app in [\"google-chrome\", \"firefox\"]", "control": {"nice": 10}}]
}
`

func TestCrossFormatEquivalence(t *testing.T) {
	v := testValidator()
	ct, err := Load([]byte(tomlCfg), FormatTOML, v)
	if err != nil {
		t.Fatalf("toml: %v", err)
	}
	cy, err := Load([]byte(yamlCfg), FormatYAML, v)
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	cj, err := Load([]byte(jsonCfg), FormatJSON, v)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if !reflect.DeepEqual(ct, cy) {
		t.Fatalf("toml != yaml\n%+v\n%+v", ct, cy)
	}
	if !reflect.DeepEqual(ct, cj) {
		t.Fatalf("toml != json\n%+v\n%+v", ct, cj)
	}
}

func TestStrictness(t *testing.T) {
	v := testValidator()
	cases := []struct {
		name   string
		data   string
		format Format
	}{
		{"toml unknown key", "version = 1\nbogus = true\n", FormatTOML},
		{"yaml unknown key", "version: 1\nbogus: true\n", FormatYAML},
		{"json unknown key", `{"version":1,"bogus":true}`, FormatJSON},
		{"json duplicate key", `{"version":1,"version":1}`, FormatJSON},
		{"toml bad duration", "version = 1\ninterval = \"3 apples\"\n", FormatTOML},
		{"bad leaf", "version = 1\nleaf = \"nonsense\"\n", FormatTOML},
		{"group none plus", "version = 1\ngroup_by = [\"none\", \"app\"]\n", FormatTOML},
		{"unknown dimension", "version = 1\ngroup_by = [\"bogusdim\"]\n", FormatTOML},
		{"unknown profile", "version = 1\n[metrics]\nprofile = \"nope\"\n", FormatTOML},
		{"bad sort dir", "version = 1\n[[sort]]\nfield=\"cpu\"\ndirection=\"sideways\"\n", FormatTOML},
	}
	for _, c := range cases {
		if _, err := Load([]byte(c.data), c.format, v); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestManagedRateSelectorRejected(t *testing.T) {
	v := testValidator()
	cfg := "version = 1\n[[managed]]\nname=\"x\"\nselector='cpu > 5'\n"
	if _, err := Load([]byte(cfg), FormatTOML, v); err == nil {
		t.Fatal("rate metric in persistent selector should be rejected (RFC §6.5)")
	}
}

func TestPresetCycle(t *testing.T) {
	v := testValidator()
	cfg := `
version = 1
[presets.a]
extends = "b"
[presets.b]
extends = "a"
`
	if _, err := Load([]byte(cfg), FormatTOML, v); err == nil {
		t.Fatal("preset inheritance cycle should be rejected")
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	v := testValidator()
	ct, err := Load([]byte(tomlCfg), FormatTOML, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []Format{FormatTOML, FormatYAML, FormatJSON} {
		data, err := Encode(ct, f)
		if err != nil {
			t.Fatalf("encode %s: %v", f, err)
		}
		back, err := Load(data, f, v)
		if err != nil {
			t.Fatalf("reload %s: %v\n%s", f, err, data)
		}
		if !reflect.DeepEqual(ct, back) {
			t.Fatalf("round trip %s mismatch:\n%+v\n%+v", f, ct, back)
		}
	}
}
