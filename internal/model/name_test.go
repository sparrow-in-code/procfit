package model

import (
	"strings"
	"testing"
)

func TestDisplayName(t *testing.T) {
	cases := []struct {
		name string
		p    Process
		want string
	}{
		{"argv0 path basename", Process{Comm: "google-chrome-s", Cmdline: []string{"/usr/bin/google-chrome-stable"}}, "google-chrome-stable"},
		{"argv0 with spaces kept", Process{Comm: "sshd", Cmdline: []string{"sshd: user [priv]"}}, "sshd: user [priv]"},
		{"exe fallback", Process{Comm: "worker", Exe: "/opt/app/bin/worker-daemon"}, "worker-daemon"},
		{"comm fallback", Process{Comm: "kworker/0:1"}, "kworker/0:1"},
		{"empty", Process{}, ""},
	}
	for _, c := range cases {
		if got := c.p.DisplayName(); got != c.want {
			t.Errorf("%s: DisplayName = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDisplayName_Cap(t *testing.T) {
	long := "/x/" + strings.Repeat("a", 200) // no separator in basename portion beyond /x/
	p := Process{Cmdline: []string{long}}
	if n := len(p.DisplayName()); n > displayNameCap {
		t.Fatalf("display name not capped: %d", n)
	}
}
