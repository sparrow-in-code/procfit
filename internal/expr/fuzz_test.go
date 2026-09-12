package expr

import "testing"

// FuzzCompile ensures the expression compiler never panics on arbitrary input
// and that a successfully compiled program evaluates without panicking either
// (RFC §26.2). Short-circuit and missing-value handling are exercised by the
// unit tests; here we only assert crash-freedom.
func FuzzCompile(f *testing.F) {
	seeds := []string{
		`cpu > 5 && comm == "x"`,
		`comm ~= "^(a|b)$"`,
		`uid in [0, 1000] || missing(nice)`,
		`!(a && b) contains "z"`,
		`[1, 2, 3]`,
		`((((`,
		`cpu >`,
		`"unterminated`,
		`20KiB < rss`,
		``,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	env := mapEnv{"cpu": Num(7), "comm": Str("x"), "uid": Num(0)}
	f.Fuzz(func(t *testing.T, src string) {
		p, err := Compile(src)
		if err != nil {
			return
		}
		_ = p.Eval(env)
		_ = p.Fields()
	})
}
