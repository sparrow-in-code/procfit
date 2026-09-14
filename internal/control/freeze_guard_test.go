package control

import "testing"

func TestFreezeSafety(t *testing.T) {
	cases := []struct {
		name    string
		cg      string
		selfCg  string
		allowed bool
	}{
		{"ordinary service", "/system.slice/app.service", "/user.slice/user-1000.slice/session-2.scope", true},
		{"ancestor of self", "/user.slice/user-1000.slice", "/user.slice/user-1000.slice/session-2.scope", false},
		{"self exactly", "/user.slice/user-1000.slice/session-2.scope", "/user.slice/user-1000.slice/session-2.scope", false},
		{"session scope", "/user.slice/user-1000.slice/session-2.scope", "", false},
		{"user manager", "/user.slice/user-1000.slice/user@1000.service", "", false},
		{"user slice", "/user.slice/user-1000.slice", "", false},
		{"root", "/", "", false},
		{"empty", "", "", false},
		{"unrelated service, self elsewhere", "/system.slice/other.service", "/system.slice/procfit.service", true},
	}
	for _, c := range cases {
		if got, _ := FreezeSafety(c.cg, c.selfCg); got != c.allowed {
			t.Errorf("%s: FreezeSafety(%q,%q)=%v, want %v", c.name, c.cg, c.selfCg, got, c.allowed)
		}
	}
}
