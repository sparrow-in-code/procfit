package procfs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestSocketInode(t *testing.T) {
	if n, ok := socketInode("socket:[12345]"); !ok || n != 12345 {
		t.Fatalf("socketInode = %d ok=%v, want 12345", n, ok)
	}
	if _, ok := socketInode("/dev/null"); ok {
		t.Fatal("non-socket target must not parse")
	}
	if _, ok := socketInode("pipe:[7]"); ok {
		t.Fatal("pipe is not a socket")
	}
}

func TestMergeNetTable(t *testing.T) {
	// tcp table: header + one LISTEN (0A, inode 1002) + one ESTAB (01, inode 1001).
	tcp := "  sl local rem st ... inode\n" +
		"0: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 1002 x\n" +
		"1: 0100007F:1539 0100007F:9999 01 00000000:00000000 00:00000000 00000000 1000 0 1001 x\n"
	m := map[uint64]sockInfo{}
	mergeNetTable(m, tcp, "tcp", 9, 3)
	if m[1002] != (sockInfo{proto: "tcp", state: "0A"}) || m[1001] != (sockInfo{proto: "tcp", state: "01"}) {
		t.Fatalf("tcp parse wrong: %+v", m)
	}
	// unix: inode at field 6, no state.
	unix := "Num RefCount Protocol Flags Type St Inode Path\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01 3001 /run/x\n"
	mergeNetTable(m, unix, "unix", 6, -1)
	if m[3001] != (sockInfo{proto: "unix"}) {
		t.Fatalf("unix parse wrong: %+v", m[3001])
	}
	// inode 0 (unbound) is skipped.
	mergeNetTable(m, "h\n0: a b 01 c d e f g 0 x\n", "tcp", 9, 3)
	if _, ok := m[0]; ok {
		t.Fatal("inode 0 must be skipped")
	}
}

func TestSocketCollector(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "net/tcp"),
		"header\n"+
			"0: L 00000000:0000 0A z z z 1000 0 1002 x\n"+ // LISTEN inode 1002
			"1: A B 01 z z z 1000 0 1001 x\n") // ESTAB inode 1001
	mustWrite(t, filepath.Join(root, "net/udp"),
		"header\n0: A B 07 z z z 1000 0 2001 x\n") // udp inode 2001
	mustWrite(t, filepath.Join(root, "net/unix"),
		"Num RefCount Protocol Flags Type St Inode Path\n"+
			"0: 2 0 10000 0001 01 3001 /run/x\n") // unix inode 3001

	// pid 100 holds all four sockets plus a regular file (ignored).
	for fd, target := range map[string]string{
		"0": "socket:[1001]", "1": "socket:[1002]", "2": "socket:[2001]",
		"3": "socket:[3001]", "4": "/dev/null",
	} {
		mustSymlink(t, target, filepath.Join(root, "100/fd", fd))
	}

	c := NewSocketCollector(root)
	procs := []model.Process{{PID: 100}, {PID: 200}} // 200 has no fd dir
	c.Collect(context.Background(), procs)

	want := map[string]float64{
		"sock-tcp": 2, "sock-listen": 1, "sock-estab": 1,
		"sock-udp": 1, "sock-unix": 1, "sock-timewait": 0, "sock-closewait": 0,
	}
	for id, exp := range want {
		if v := procs[0].Metric(model.MetricID(id)); !v.Present() || v.V != exp {
			t.Fatalf("pid100 %s = %+v, want %v", id, v, exp)
		}
	}
	// A process whose fd dir can't be read is unavailable, not zero.
	if v := procs[1].Metric("sock-tcp"); v.Present() {
		t.Fatalf("pid200 (no fd dir) must be unavailable, got %+v", v)
	}
}
