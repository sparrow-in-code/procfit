package collect

import (
	"context"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// Sampler produces successive Snapshot generations from a ProcessSource,
// computing gauges directly and rates by diffing against the previous
// generation. It is deterministic given a fake clock and source.
type Sampler struct {
	src ports.ProcessSource
	clk ports.Clock

	prev        map[string]ports.ProcStat
	prevThreads map[string]ports.ThreadStat
	prevMono    int64 // nanoseconds
	hasPrev     bool
	gen         int
}

// NewSampler constructs a sampler over the given source and clock.
func NewSampler(src ports.ProcessSource, clk ports.Clock) *Sampler {
	return &Sampler{src: src, clk: clk, prev: map[string]ports.ProcStat{}, prevThreads: map[string]ports.ThreadStat{}}
}

// Sample reads the source once and returns a normalized snapshot with the
// requested metrics computed. The first sample (and any newly-appeared process)
// yields warming_up for rate metrics rather than a bogus zero (RFC §13.6).
func (s *Sampler) Sample(ctx context.Context, want []model.MetricID) (Snapshot, error) {
	stats, err := s.src.List(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	nowMono := s.clk.NowMono().Nanoseconds()
	wall := s.clk.Now()
	bootID, _ := s.src.BootID()

	elapsedSec := 0.0
	if s.hasPrev {
		elapsedSec = float64(nowMono-s.prevMono) / 1e9
	}
	sc := sysctx{
		hz:       float64(s.src.ClockTicksPerSec()),
		cpus:     float64(s.src.OnlineCPUs()),
		bootUnix: s.src.BootTimeUnix(),
		wallNow:  wall,
	}

	procs := make([]model.Process, 0, len(stats))
	next := make(map[string]ports.ProcStat, len(stats))
	nextThreads := map[string]ports.ThreadStat{}
	for _, st := range stats {
		key := st.ID.Key()
		next[key] = st
		prev, hadPrev := s.prev[key]
		p := s.buildProcess(st, prev, hadPrev, elapsedSec, sc, want)
		p.Threads = s.buildThreads(st, elapsedSec, sc, want, nextThreads)
		procs = append(procs, p)
	}
	s.prevThreads = nextThreads

	s.gen++
	snap := Snapshot{
		Generation: s.gen,
		BootID:     bootID,
		WallTime:   wall,
		Processes:  procs,
	}
	if s.hasPrev {
		snap.Elapsed = time.Duration(nowMono - s.prevMono)
	}
	s.prev = next
	s.prevMono = nowMono
	s.hasPrev = true
	return snap, nil
}

// buildProcess normalizes one raw stat into a model.Process with requested
// metrics attached.
func (s *Sampler) buildProcess(st, prev ports.ProcStat, hadPrev bool, elapsedSec float64, sc sysctx, want []model.MetricID) model.Process {
	p := model.Process{
		ID: st.ID, PID: st.PID, PPID: st.PPID, PGID: st.PGID, SID: st.SID, TGID: st.TGID,
		UID: st.UID, EUID: st.EUID, GID: st.GID, EGID: st.EGID,
		Comm: st.Comm, Cmdline: st.Cmdline, CmdlineAvail: st.CmdlineAvail,
		Exe: st.Exe, Cwd: st.Cwd, CgroupPath: st.CgroupPath, Wchan: st.Wchan, Hostname: st.Hostname,
		Namespaces: st.Namespaces, State: st.State,
		Nice: st.Nice, NiceAvail: st.NiceAvail, StartTicks: st.StartTicks,
		NumThreads: st.NumThreads,
	}
	for _, id := range want {
		p.SetMetric(id, computeMetric(id, st, prev, hadPrev, elapsedSec, sc))
	}
	return p
}

// buildThreads normalizes a process's enumerated threads, computing per-thread
// cpu-family rates against the previous generation. Non-cpu metrics are not
// meaningful per thread (e.g. rss is shared), so they render unavailable.
func (s *Sampler) buildThreads(st ports.ProcStat, elapsedSec float64, sc sysctx, want []model.MetricID, nextThreads map[string]ports.ThreadStat) []model.Thread {
	if len(st.Threads) == 0 {
		return nil
	}
	threads := make([]model.Thread, 0, len(st.Threads))
	for _, ts := range st.Threads {
		id := model.ThreadInstanceID{Process: st.ID, TID: ts.TID, StartTime: ts.StartTicks}
		key := id.Key()
		nextThreads[key] = ts
		prev, hadPrev := s.prevThreads[key]
		th := model.Thread{ID: id, Comm: ts.Comm, State: ts.State, Metrics: map[model.MetricID]model.MetricValue{}}
		for _, mid := range want {
			th.Metrics[mid] = threadMetric(mid, ts, prev, hadPrev, elapsedSec, sc)
		}
		threads = append(threads, th)
	}
	return threads
}

// threadCounters extracts the cumulative CPU ticks for the per-thread rate
// metrics; only cpu-family metrics are supported at thread scope.
var threadCounters = map[model.MetricID]func(ports.ThreadStat) uint64{
	"cpu":        func(t ports.ThreadStat) uint64 { return t.UTimeTicks + t.STimeTicks },
	"cpu-user":   func(t ports.ThreadStat) uint64 { return t.UTimeTicks },
	"cpu-system": func(t ports.ThreadStat) uint64 { return t.STimeTicks },
}

func threadMetric(id model.MetricID, cur, prev ports.ThreadStat, hadPrev bool, elapsedSec float64, sc sysctx) model.MetricValue {
	get, ok := threadCounters[id]
	if !ok {
		return model.Unavailable[float64](model.Disabled, "thread")
	}
	if !hadPrev || elapsedSec <= 0 {
		return model.Unavailable[float64](model.WarmingUp, "thread")
	}
	c, p := get(cur), get(prev)
	if c < p {
		return model.Unavailable[float64](model.WarmingUp, "thread")
	}
	return model.NewValue(cpuConv(float64(c-p)/elapsedSec, sc), model.Derived, "thread")
}

// computeMetric produces one metric value for a process using the rate/gauge
// spec tables.
func computeMetric(id model.MetricID, cur, prev ports.ProcStat, hadPrev bool, elapsedSec float64, sc sysctx) model.MetricValue {
	if spec, ok := rateSpecs[id]; ok {
		return computeRate(spec, cur, prev, hadPrev, elapsedSec, sc)
	}
	if spec, ok := gaugeSpecs[id]; ok {
		v, avail, q := spec.get(cur, sc)
		if avail != model.Available {
			return model.Unavailable[float64](avail, "proc")
		}
		return model.NewValue(v, q, "proc")
	}
	return model.Unavailable[float64](model.Disabled, "proc")
}

// computeRate applies rate semantics: warming up without a prior sample or valid
// elapsed time, discarding negative deltas (counter reset/replacement).
func computeRate(spec rateSpec, cur, prev ports.ProcStat, hadPrev bool, elapsedSec float64, sc sysctx) model.MetricValue {
	curV, curAvail := spec.get(cur)
	if curAvail != model.Available {
		return model.Unavailable[float64](curAvail, "proc")
	}
	if !hadPrev || elapsedSec <= 0 {
		return model.Unavailable[float64](model.WarmingUp, "proc")
	}
	prevV, prevAvail := spec.get(prev)
	if prevAvail != model.Available {
		return model.Unavailable[float64](model.WarmingUp, "proc")
	}
	if curV < prevV {
		// Counter went backwards: reset/replacement; discard this delta.
		return model.Unavailable[float64](model.WarmingUp, "proc")
	}
	perSec := float64(curV-prevV) / elapsedSec
	return model.NewValue(spec.conv(perSec, sc), model.Derived, "proc")
}
