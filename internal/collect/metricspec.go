package collect

import (
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// sysctx carries per-sample system constants needed for metric conversion.
type sysctx struct {
	hz       float64
	cpus     float64
	bootUnix int64
	wallNow  time.Time
}

// rateSpec describes how to extract a cumulative counter and convert its
// per-second delta into a metric value. Keeping these as data (a map) means the
// sampler loop stays small and adding a light rate metric is a one-line entry
// (Open/Closed within the proc collector).
type rateSpec struct {
	// get returns the raw cumulative counter and whether it is readable.
	get func(ports.ProcStat) (uint64, model.Availability)
	// conv converts a per-second delta into the final metric value.
	conv func(perSec float64, s sysctx) float64
}

// gaugeSpec extracts an absolute value from the current sample.
type gaugeSpec struct {
	get func(cur ports.ProcStat, s sysctx) (val float64, avail model.Availability, q model.Quality)
}

func cpuConv(perSec float64, s sysctx) float64 {
	if s.hz == 0 {
		return 0
	}
	return perSec / s.hz * 100
}

func ioAvail(p ports.ProcStat, v uint64) (uint64, model.Availability) {
	if p.IOAvail != model.Available {
		return 0, p.IOAvail
	}
	return v, model.Available
}

// rateSpecs maps rate metric ids to their extraction/conversion.
var rateSpecs = map[model.MetricID]rateSpec{
	"cpu": {get: func(p ports.ProcStat) (uint64, model.Availability) {
		return p.UTimeTicks + p.STimeTicks, model.Available
	}, conv: cpuConv},
	"cpu-user":   {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.UTimeTicks, model.Available }, conv: cpuConv},
	"cpu-system": {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.STimeTicks, model.Available }, conv: cpuConv},
	"cpu-normalized": {get: func(p ports.ProcStat) (uint64, model.Availability) {
		return p.UTimeTicks + p.STimeTicks, model.Available
	},
		conv: func(perSec float64, s sysctx) float64 {
			v := cpuConv(perSec, s)
			if s.cpus > 0 {
				v /= s.cpus
			}
			return v
		}},
	"minor-faults":      {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.MinFlt, model.Available }, conv: identityRate},
	"major-faults":      {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.MajFlt, model.Available }, conv: identityRate},
	"ctxsw-voluntary":   {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.VoluntaryCtxt, model.Available }, conv: identityRate},
	"ctxsw-involuntary": {get: func(p ports.ProcStat) (uint64, model.Availability) { return p.InvoluntaryCtxt, model.Available }, conv: identityRate},
	"disk-rbps":         {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.ReadBytes) }, conv: identityRate},
	"disk-wbps":         {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.WriteBytes) }, conv: identityRate},
	"io-rchar":          {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.RChar) }, conv: identityRate},
	"io-wchar":          {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.WChar) }, conv: identityRate},
	"read-syscalls":     {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.Syscr) }, conv: identityRate},
	"write-syscalls":    {get: func(p ports.ProcStat) (uint64, model.Availability) { return ioAvail(p, p.IO.Syscw) }, conv: identityRate},
	// blkio-delay: fraction of wall time the task spent blocked on block I/O
	// (delayacct ticks → % via the CPU-tick conversion). A key load-average driver.
	"blkio-delay": {get: func(p ports.ProcStat) (uint64, model.Availability) {
		if p.BlkioAvail != "" && p.BlkioAvail != model.Available {
			return 0, p.BlkioAvail
		}
		return p.BlkioTicks, model.Available
	}, conv: cpuConv},
}

func identityRate(perSec float64, _ sysctx) float64 { return perSec }

// gaugeSpecs maps gauge metric ids to their extraction.
var gaugeSpecs = map[model.MetricID]gaugeSpec{
	"rss": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.RSSBytes), model.Available, model.Exact
	}},
	"vsz": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.VSZBytes), model.Available, model.Exact
	}},
	"threads": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.NumThreads), model.Available, model.Exact
	}},
	"swap": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.SwapBytes), model.Available, model.Exact
	}},
	"mem-peak": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.RSSPeakBytes), model.Available, model.Exact
	}},
	"rss-anon": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.RSSAnonBytes), model.Available, model.Exact
	}},
	"rss-file": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.RSSFileBytes), model.Available, model.Exact
	}},
	"rss-shmem": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		return float64(p.RSSShmemBytes), model.Available, model.Exact
	}},
	"pnice": {get: func(p ports.ProcStat, _ sysctx) (float64, model.Availability, model.Quality) {
		if p.NiceAvail != model.Available {
			return 0, p.NiceAvail, ""
		}
		return float64(p.Nice), model.Available, model.Exact
	}},
	"age": {get: func(p ports.ProcStat, s sysctx) (float64, model.Availability, model.Quality) {
		if s.bootUnix == 0 || s.hz == 0 {
			return 0, model.Unsupported, ""
		}
		startWall := float64(s.bootUnix) + float64(p.StartTicks)/s.hz
		age := s.wallNow.Sub(time.Unix(int64(startWall), 0)).Seconds()
		if age < 0 {
			age = 0
		}
		return age, model.Available, model.Derived
	}},
}
