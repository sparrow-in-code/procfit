package procfs

import (
	"strings"
	"testing"
)

func TestParseStat_SimpleComm(t *testing.T) {
	line := "1234 (bash) S 1000 1234 1234 0 -1 4194560 100 0 0 0 5 3 0 0 20 0 1 0 987654 12345678 200 0 0 0 0 0 0 0 0 0 0 0 0 0"
	info, err := parseStat([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if info.PID != 1234 || info.Comm != "bash" || info.State != 'S' {
		t.Fatalf("basic fields wrong: %+v", info)
	}
	if info.PPID != 1000 || info.PGID != 1234 || info.SID != 1234 {
		t.Fatalf("ppid/pgid/sid wrong: %+v", info)
	}
	if info.MinFlt != 100 || info.UTime != 5 || info.STime != 3 {
		t.Fatalf("counters wrong: minflt=%d utime=%d stime=%d", info.MinFlt, info.UTime, info.STime)
	}
	// In this crafted line field 18 (priority) = 20 and field 19 (nice) = 0.
	if info.Nice != 0 {
		t.Fatalf("nice = %d, want 0", info.Nice)
	}
	if info.NumThreads != 1 || info.StartTime != 987654 || info.VSize != 12345678 {
		t.Fatalf("threads/start/vsize wrong: %+v", info)
	}
	if info.RSSPages != 200 {
		t.Fatalf("rss pages = %d, want 200", info.RSSPages)
	}
}

func TestParseStat_BlkioDelay(t *testing.T) {
	// rest[] index = field-3, so field 42 (delayacct_blkio_ticks) is rest[39].
	rest := make([]string, 40)
	for i := range rest {
		rest[i] = "0"
	}
	rest[0] = "S"    // field 3: state
	rest[39] = "777" // field 42: blkio delay ticks
	line := "1234 (bash) " + strings.Join(rest, " ")
	info, err := parseStat([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if info.BlkioTicks != 777 {
		t.Fatalf("blkio ticks = %d, want 777", info.BlkioTicks)
	}
}

func TestParseStat_CommWithSpacesAndParens(t *testing.T) {
	// A pathological comm containing spaces and parentheses (RFC §14.1).
	line := "42 (weird )(name) with spaces) R 7 42 42 0 -1 0 0 0 0 0 11 22 0 0 20 0 3 0 555 999 10 0 0 0 0 0 0 0 0 0 0 0 0 0"
	info, err := parseStat([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	if info.PID != 42 {
		t.Fatalf("pid = %d, want 42", info.PID)
	}
	if info.Comm != "weird )(name) with spaces" {
		t.Fatalf("comm parsed wrong: %q", info.Comm)
	}
	if info.State != 'R' {
		t.Fatalf("state = %c, want R", info.State)
	}
	if info.UTime != 11 || info.STime != 22 || info.NumThreads != 3 || info.StartTime != 555 {
		t.Fatalf("fields after pathological comm wrong: %+v", info)
	}
}

func TestParseStat_Malformed(t *testing.T) {
	if _, err := parseStat([]byte("no parens here")); err == nil {
		t.Fatal("expected error for missing comm parentheses")
	}
	if _, err := parseStat([]byte("notanumber (x) R")); err == nil {
		t.Fatal("expected error for bad pid")
	}
}

func TestParseNSInode(t *testing.T) {
	n, ok := parseNSInode("pid:[4026531836]")
	if !ok || n != 4026531836 {
		t.Fatalf("ns inode = %d ok=%v", n, ok)
	}
	if _, ok := parseNSInode("garbage"); ok {
		t.Fatal("garbage should not parse")
	}
}

func TestParseIO(t *testing.T) {
	data := "rchar: 100\nwchar: 200\nread_bytes: 4096\nwrite_bytes: 8192\nsyscr: 5\nsyscw: 6\n"
	m := parseIO([]byte(data))
	if m["read_bytes"] != 4096 || m["wchar"] != 200 || m["syscr"] != 5 {
		t.Fatalf("io parse wrong: %+v", m)
	}
}

func TestParseStatusAndBtime(t *testing.T) {
	status := "Name:\tbash\nUid:\t1000\t1000\t1000\t1000\nGid:\t1000\t1000\t1000\t1000\n" +
		"VmHWM:\t   2048 kB\nRssAnon:\t 1024 kB\nRssFile:\t  512 kB\nRssShmem:\t  256 kB\nVmSwap:\t  128 kB\n" +
		"voluntary_ctxt_switches:\t42\nnonvoluntary_ctxt_switches:\t7\n"
	si := parseStatus([]byte(status))
	if si.UID != 1000 || si.GID != 1000 || si.VolCtx != 42 || si.InvolCtx != 7 {
		t.Fatalf("status identity/ctxsw wrong: %+v", si)
	}
	if si.RSSPeak != 2048*1024 || si.RSSAnon != 1024*1024 || si.RSSFile != 512*1024 ||
		si.RSSShmem != 256*1024 || si.Swap != 128*1024 {
		t.Fatalf("status memory (bytes) wrong: %+v", si)
	}
	if b := parseBtime([]byte("cpu 1 2 3\nbtime 1600000000\nprocesses 5\n")); b != 1600000000 {
		t.Fatalf("btime = %d", b)
	}
}
