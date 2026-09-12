package expr

import (
	"fmt"
	"strings"
)

// applySuffix converts a numeric literal with an optional unit suffix into a
// canonical value: byte sizes become bytes, durations become seconds, and a
// bare number is unchanged (RFC §10.3).
func applySuffix(base float64, suffix string) (float64, error) {
	if suffix == "" {
		return base, nil
	}
	if mult, ok := durationSeconds(suffix); ok {
		return base * mult, nil
	}
	if mult, ok := byteSize(suffix); ok {
		return base * mult, nil
	}
	return 0, fmt.Errorf("expr: unknown numeric suffix %q", suffix)
}

// durationSeconds returns the seconds-per-unit for Go-like duration suffixes.
func durationSeconds(s string) (float64, bool) {
	switch s {
	case "ms":
		return 0.001, true
	case "s":
		return 1, true
	case "m":
		return 60, true
	case "h":
		return 3600, true
	default:
		return 0, false
	}
}

// byteSize returns the bytes-per-unit for size suffixes. An "i" (KiB/MiB/...)
// selects binary (1024) scaling; otherwise decimal (1000) is used. A trailing
// "B" is optional (K, KB both decimal kilo).
func byteSize(s string) (float64, bool) {
	binary := strings.Contains(s, "i")
	letter := s[0]
	var exp float64
	switch letter {
	case 'K':
		exp = 1
	case 'M':
		exp = 2
	case 'G':
		exp = 3
	case 'T':
		exp = 4
	case 'P':
		exp = 5
	default:
		return 0, false
	}
	// Validate the remainder is one of "", "B", "iB".
	rest := s[1:]
	if rest != "" && rest != "B" && rest != "iB" {
		return 0, false
	}
	step := 1000.0
	if binary {
		step = 1024.0
	}
	mult := 1.0
	for i := 0; i < int(exp); i++ {
		mult *= step
	}
	return mult, true
}
