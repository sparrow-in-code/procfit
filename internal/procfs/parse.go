// Package procfs is the Linux ProcessSource adapter: it reads and parses /proc
// to satisfy the ports.ProcessSource interface (RFC §14.1). The parsers are
// pure and OS-independent so they can be unit-tested and fuzzed on any platform;
// the file I/O lives in the build-tagged source file.
package procfs

import (
	"fmt"
	"strconv"
	"strings"
)

// statInfo holds the fields we consume from /proc/<pid>/stat.
type statInfo struct {
	PID        int
	Comm       string
	State      rune
	PPID       int
	PGID       int
	SID        int
	MinFlt     uint64
	MajFlt     uint64
	UTime      uint64
	STime      uint64
	Nice       int
	NumThreads int
	StartTime  uint64
	VSize      uint64
	RSSPages   int64
}

// parseStat parses one /proc/<pid>/stat line. The comm field (field 2) is
// enclosed in parentheses and may itself contain spaces and parentheses, so we
// locate the LAST ')' rather than naively splitting on whitespace (RFC §14.1).
func parseStat(data []byte) (statInfo, error) {
	s := string(data)
	open := strings.IndexByte(s, '(')
	closeIdx := strings.LastIndexByte(s, ')')
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return statInfo{}, fmt.Errorf("procfs: malformed stat: no comm parentheses")
	}
	var info statInfo
	pidStr := strings.TrimSpace(s[:open])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return statInfo{}, fmt.Errorf("procfs: bad pid %q: %w", pidStr, err)
	}
	info.PID = pid
	info.Comm = s[open+1 : closeIdx]

	rest := strings.Fields(s[closeIdx+1:])
	// rest[0] is field 3 (state); field N maps to rest[N-3].
	get := func(field int) (string, bool) {
		idx := field - 3
		if idx < 0 || idx >= len(rest) {
			return "", false
		}
		return rest[idx], true
	}
	if v, ok := get(3); ok && len(v) > 0 {
		info.State = rune(v[0])
	}
	info.PPID = atoiField(get, 4)
	info.PGID = atoiField(get, 5)
	info.SID = atoiField(get, 6)
	info.MinFlt = auintField(get, 10)
	info.MajFlt = auintField(get, 12)
	info.UTime = auintField(get, 14)
	info.STime = auintField(get, 15)
	info.Nice = atoiField(get, 19)
	info.NumThreads = atoiField(get, 20)
	info.StartTime = auintField(get, 22)
	info.VSize = auintField(get, 23)
	info.RSSPages = int64(auintField(get, 24))
	return info, nil
}

func atoiField(get func(int) (string, bool), field int) int {
	if v, ok := get(field); ok {
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func auintField(get func(int) (string, bool), field int) uint64 {
	if v, ok := get(field); ok {
		n, _ := strconv.ParseUint(v, 10, 64)
		return n
	}
	return 0
}

// parseIO parses /proc/<pid>/io into a map of counter name to value.
func parseIO(data []byte) map[string]uint64 {
	out := make(map[string]uint64)
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err == nil {
			out[strings.TrimSpace(k)] = n
		}
	}
	return out
}

// parseNSInode extracts the inode from an ns symlink target like
// "pid:[4026531836]".
func parseNSInode(target string) (uint64, bool) {
	open := strings.IndexByte(target, '[')
	closeIdx := strings.IndexByte(target, ']')
	if open < 0 || closeIdx < 0 || closeIdx < open {
		return 0, false
	}
	n, err := strconv.ParseUint(target[open+1:closeIdx], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseStatusUIDGID extracts the real UID and GID from /proc/<pid>/status.
func parseStatus(data []byte) (uid, gid uint32, volCtx, involCtx uint64) {
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "Uid:"):
			uid = firstUint32(line[4:])
		case strings.HasPrefix(line, "Gid:"):
			gid = firstUint32(line[4:])
		case strings.HasPrefix(line, "voluntary_ctxt_switches:"):
			volCtx = firstUint64(line[len("voluntary_ctxt_switches:"):])
		case strings.HasPrefix(line, "nonvoluntary_ctxt_switches:"):
			involCtx = firstUint64(line[len("nonvoluntary_ctxt_switches:"):])
		}
	}
	return
}

func firstUint32(s string) uint32 {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0
	}
	n, _ := strconv.ParseUint(f[0], 10, 32)
	return uint32(n)
}

func firstUint64(s string) uint64 {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0
	}
	n, _ := strconv.ParseUint(f[0], 10, 64)
	return n
}

// parseBtime extracts the btime (boot unix time) from /proc/stat.
func parseBtime(data []byte) int64 {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "btime ") {
			n, _ := strconv.ParseInt(strings.TrimSpace(line[6:]), 10, 64)
			return n
		}
	}
	return 0
}
