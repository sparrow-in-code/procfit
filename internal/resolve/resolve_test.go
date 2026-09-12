package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestParsePasswd(t *testing.T) {
	data := "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/sh\nbad-line\n#comment\n"
	m := parsePasswd([]byte(data))
	if m[0] != "root" || m[1000] != "alice" {
		t.Fatalf("passwd parse wrong: %+v", m)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
}

func TestUserResolver(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "passwd")
	os.WriteFile(pw, []byte("root:x:0:0::/root:/bin/sh\nbob:x:1001:1001::/home/bob:/bin/sh\n"), 0o644)
	r := NewUserResolver(pw)
	if r.ID() != "user" {
		t.Fatal("id")
	}
	p := &model.Process{UID: 1001}
	r.Resolve(p)
	if p.User != "bob" {
		t.Fatalf("user = %q, want bob", p.User)
	}
	// Unknown uid leaves User empty (no fabrication).
	q := &model.Process{UID: 4242}
	r.Resolve(q)
	if q.User != "" {
		t.Fatalf("unknown uid should stay empty, got %q", q.User)
	}
}

func TestUserResolver_MissingFile(t *testing.T) {
	r := NewUserResolver(filepath.Join(t.TempDir(), "nope"))
	p := &model.Process{UID: 0}
	r.Resolve(p) // must not panic; leaves User empty
	if p.User != "" {
		t.Fatal("missing passwd should leave user empty")
	}
}

func TestSystemdResolver(t *testing.T) {
	r := NewSystemdResolver()
	cases := map[string]string{
		"/system.slice/nginx.service":                         "nginx.service",
		"/user.slice/user-1000.slice/session-2.scope":         "session-2.scope",
		"/system.slice/system-getty.slice/getty@tty1.service": "getty@tty1.service",
		"/user.slice/user-1000.slice":                         "user-1000.slice",
		"/":                                                   "",
	}
	for cg, want := range cases {
		p := &model.Process{CgroupPath: cg}
		r.Resolve(p)
		if p.SystemdUnit != want {
			t.Errorf("cgroup %q => unit %q, want %q", cg, p.SystemdUnit, want)
		}
	}
}

func TestSystemdResolver_DoesNotOverwrite(t *testing.T) {
	r := NewSystemdResolver()
	p := &model.Process{CgroupPath: "/system.slice/nginx.service", SystemdUnit: "preset.service"}
	r.Resolve(p)
	if p.SystemdUnit != "preset.service" {
		t.Fatal("resolver must not overwrite an existing unit")
	}
}
