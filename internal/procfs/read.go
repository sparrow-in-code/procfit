package procfs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// readProcess reads one process directory into a ProcStat. It returns ok=false
// if the essential stat file is unreadable (the process vanished).
func (s *Source) readProcess(pid int) (ports.ProcStat, bool) {
	dir := filepath.Join(s.root, itoa(pid))
	statData, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return ports.ProcStat{}, false
	}
	info, err := parseStat(statData)
	if err != nil {
		return ports.ProcStat{}, false
	}

	ns := s.readNamespaces(dir)
	st := ports.ProcStat{
		ID: model.ProcessInstanceID{
			BootID:     s.bootID,
			PIDNSInode: ns[model.NSPID],
			PID:        info.PID,
			StartTime:  info.StartTime,
		},
		PID:        info.PID,
		PPID:       info.PPID,
		PGID:       info.PGID,
		SID:        info.SID,
		Comm:       info.Comm,
		State:      model.ProcessState{Code: info.State},
		Nice:       info.Nice,
		NiceAvail:  model.Available,
		NumThreads: info.NumThreads,
		StartTicks: info.StartTime,
		UTimeTicks: info.UTime,
		STimeTicks: info.STime,
		MinFlt:     info.MinFlt,
		MajFlt:     info.MajFlt,
		VSZBytes:   info.VSize,
		RSSBytes:   uint64(info.RSSPages) * uint64(s.pageSize),
		Namespaces: ns,
	}

	s.fillStatus(dir, &st)
	s.fillIO(dir, &st)
	st.Cmdline, st.CmdlineAvail = s.readCmdline(dir)
	st.Exe, st.ExeAvail = s.readLinkAvail(filepath.Join(dir, "exe"))
	st.CgroupPath = s.readCgroup(dir)
	return st, true
}

func (s *Source) fillStatus(dir string, st *ports.ProcStat) {
	data, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		return
	}
	uid, gid, vol, invol := parseStatus(data)
	st.UID, st.EUID = uid, uid
	st.GID, st.EGID = gid, gid
	st.VoluntaryCtxt = vol
	st.InvoluntaryCtxt = invol
}

func (s *Source) fillIO(dir string, st *ports.ProcStat) {
	data, err := os.ReadFile(filepath.Join(dir, "io"))
	if err != nil {
		if os.IsPermission(err) {
			st.IOAvail = model.PermissionDenied
		} else {
			st.IOAvail = model.ReadError
		}
		return
	}
	m := parseIO(data)
	st.IO = ports.ProcIO{
		ReadBytes:           m["read_bytes"],
		WriteBytes:          m["write_bytes"],
		RChar:               m["rchar"],
		WChar:               m["wchar"],
		Syscr:               m["syscr"],
		Syscw:               m["syscw"],
		CancelledWriteBytes: m["cancelled_write_bytes"],
	}
	st.IOAvail = model.Available
}

func (s *Source) readNamespaces(dir string) model.NamespaceSet {
	ns := make(model.NamespaceSet)
	for _, t := range model.AllNamespaceTypes {
		target, err := os.Readlink(filepath.Join(dir, "ns", string(t)))
		if err != nil {
			continue
		}
		if ino, ok := parseNSInode(target); ok {
			ns[t] = ino
		}
	}
	return ns
}

func (s *Source) readCmdline(dir string) ([]string, model.Availability) {
	data, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		if os.IsPermission(err) {
			return nil, model.PermissionDenied
		}
		return nil, model.ReadError
	}
	trimmed := strings.TrimRight(string(data), "\x00")
	if trimmed == "" {
		return nil, model.Available // genuinely empty (e.g. kernel thread)
	}
	return strings.Split(trimmed, "\x00"), model.Available
}

func (s *Source) readLinkAvail(path string) (string, model.Availability) {
	target, err := os.Readlink(path)
	if err != nil {
		if os.IsPermission(err) {
			return "", model.PermissionDenied
		}
		return "", model.ReadError
	}
	return target, model.Available
}

func (s *Source) readCgroup(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "cgroup"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		// cgroup v2 line looks like "0::/user.slice/...".
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::")
		}
	}
	return ""
}

func itoa(n int) string {
	// small local helper to avoid importing strconv in two files' hot paths
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
