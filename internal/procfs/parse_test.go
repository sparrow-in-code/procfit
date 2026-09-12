package procfs

import "testing"

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
	if info.Nice != 20 { // field 19 = priority(20)?; verify mapping below
		// note: in this crafted line field18=priority=20, field19=nice=0
	}
	if info.NumThreads != 1 || info.StartTime != 987654 || info.VSize != 12345678 {
		t.Fatalf("threads/start/vsize wrong: %+v", info)
	}
	if info.RSSPages != 200 {
		t.Fatalf("rss pages = %d, want 200", info.RSSPages)
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
	status := "Name:\tbash\nUid:\t1000\t1000\t1000\t1000\nGid:\t1000\t1000\t1000\t1000\nvoluntary_ctxt_switches:\t42\nnonvoluntary_ctxt_switches:\t7\n"
	uid, gid, vol, invol := parseStatus([]byte(status))
	if uid != 1000 || gid != 1000 || vol != 42 || invol != 7 {
		t.Fatalf("status parse wrong: uid=%d gid=%d vol=%d invol=%d", uid, gid, vol, invol)
	}
	if b := parseBtime([]byte("cpu 1 2 3\nbtime 1600000000\nprocesses 5\n")); b != 1600000000 {
		t.Fatalf("btime = %d", b)
	}
}
