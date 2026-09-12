package expr

import "testing"

type mapEnv map[string]Value

func (m mapEnv) Lookup(f string) Value {
	if v, ok := m[f]; ok {
		return v
	}
	return Missing
}

func evalOK(t *testing.T, src string, env Env) bool {
	t.Helper()
	p, err := Compile(src)
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return p.Eval(env)
}

func TestCompareNumbersAndPrecedence(t *testing.T) {
	env := mapEnv{"cpu": Num(6), "wakeups": Num(50), "disk-wbps": Num(100)}
	cases := map[string]bool{
		"cpu > 5":                   true,
		"cpu >= 6 && wakeups < 100": true,
		"cpu > 5 || wakeups > 100":  true,
		"(cpu > 5 || wakeups > 100) && disk-wbps < 600": true,
		"cpu < 5 && wakeups > 100":                      false,
		"!(cpu > 5)":                                    false,
	}
	for src, want := range cases {
		if got := evalOK(t, src, env); got != want {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
}

func TestMissingIsFalse(t *testing.T) {
	env := mapEnv{"cpu": Num(6)}
	// nosuch is missing; every ordinary comparison including != is false.
	for _, src := range []string{"nosuch > 0", "nosuch == 0", "nosuch != 0", "nosuch < 100"} {
		if evalOK(t, src, env) {
			t.Errorf("%q should be false for missing field", src)
		}
	}
	if !evalOK(t, "missing(nosuch)", env) {
		t.Error("missing(nosuch) should be true")
	}
	if !evalOK(t, "exists(cpu)", env) {
		t.Error("exists(cpu) should be true")
	}
	if evalOK(t, "exists(nosuch)", env) {
		t.Error("exists(nosuch) should be false")
	}
}

func TestStringOpsAndRegex(t *testing.T) {
	env := mapEnv{"comm": Str("chrome"), "cmdline": Str("/usr/bin/idea --args")}
	cases := map[string]bool{
		`comm == "chrome"`:              true,
		`comm != "firefox"`:             true,
		`comm ~= "^(chrome|chromium)$"`: true,
		`comm !~ "firefox"`:             true,
		`cmdline contains "idea"`:       true,
		`cmdline contains "eclipse"`:    false,
		`comm in ["firefox", "chrome"]`: true,
		`comm not in ["firefox"]`:       true,
	}
	for src, want := range cases {
		if got := evalOK(t, src, env); got != want {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
}

func TestSizeAndDurationLiterals(t *testing.T) {
	env := mapEnv{"rss": Num(20 * 1024 * 1024), "age": Num(120)}
	cases := map[string]bool{
		"rss > 10M":    true,  // 10 * 1000^2 = 10,000,000 < 20MiB
		"rss > 20MiB":  false, // equal, not greater
		"rss >= 20MiB": true,
		"rss < 1G":     true,
		"age > 1m":     true, // 120s > 60s
		"age >= 2m":    true,
		"age > 1h":     false,
	}
	for src, want := range cases {
		if got := evalOK(t, src, env); got != want {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
}

func TestBareBooleanField(t *testing.T) {
	env := mapEnv{"managed": Bool(true), "drifted": Bool(false)}
	if !evalOK(t, "managed", env) {
		t.Error("bare true bool field should be truthy")
	}
	if evalOK(t, "drifted", env) {
		t.Error("bare false bool field should be falsy")
	}
	if !evalOK(t, "managed && !drifted", env) {
		t.Error("managed && !drifted should be true")
	}
}

func TestCompileErrors(t *testing.T) {
	bad := []string{
		`comm == "unterminated`,
		`cpu >`,
		`foo(bar)`,      // unknown function
		`comm ~= 5`,     // regex needs string
		`comm ~= "("`,   // invalid regex
		`cpu > 5 extra`, // trailing input
		`(cpu > 5`,      // unbalanced paren
		`[1,2`,          // unterminated list
		`5 @ 3`,         // bad char
		`cpu > 5K5`,     // bad numeric suffix
	}
	for _, src := range bad {
		if _, err := Compile(src); err == nil {
			t.Errorf("expected compile error for %q", src)
		}
	}
}

func TestShortCircuit(t *testing.T) {
	// If && did not short-circuit, evaluating the right side is still safe here,
	// but we assert the logical result and that missing right side yields false.
	env := mapEnv{"cpu": Num(1)}
	if evalOK(t, "cpu > 5 && nosuch > 0", env) {
		t.Error("false && _ should be false")
	}
	if !evalOK(t, "cpu > 0 || nosuch > 0", env) {
		t.Error("true || _ should be true")
	}
}

func TestValidateFields(t *testing.T) {
	p, err := Compile(`cpu > 5 && comm == "x"`)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(map[string]bool{"cpu": true, "comm": true}); err != nil {
		t.Fatalf("valid fields rejected: %v", err)
	}
	if err := p.Validate(map[string]bool{"cpu": true}); err == nil {
		t.Fatal("unknown field should fail validation")
	}
	if got := p.Fields(); len(got) != 2 {
		t.Fatalf("Fields() = %v", got)
	}
}
