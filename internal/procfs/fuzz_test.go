package procfs

import "testing"

// FuzzParseStat ensures the /proc/PID/stat parser never panics on arbitrary
// input, including pathological comm fields (RFC §26.2).
func FuzzParseStat(f *testing.F) {
	f.Add("1234 (bash) S 1 1 1 0 -1 0 0 0 0 0 1 1 0 0 20 0 1 0 5 100 2 0 0 0 0 0 0 0 0 0 0 0 0 0")
	f.Add("42 (weird )(name) with spaces) R 7 42 42")
	f.Add("")
	f.Add("garbage without parens")
	f.Add("((((")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseStat([]byte(s)) // must not panic
	})
}

// FuzzParseIO ensures the io parser never panics.
func FuzzParseIO(f *testing.F) {
	f.Add("rchar: 1\nwchar: 2\n")
	f.Add(":::\n\n\x00")
	f.Fuzz(func(t *testing.T, s string) {
		_ = parseIO([]byte(s))
		_ = parseStatusData(s)
	})
}

func parseStatusData(s string) uint32 {
	u, _, _, _ := parseStatus([]byte(s))
	return u
}
