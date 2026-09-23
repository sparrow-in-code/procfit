package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/netikras/procfit/internal/model"
)

// gpuWindow is the self-contained measurement window for GPU engine utilization:
// the collector reads per-client engine busy-time counters, waits, and reads
// again to derive a % over the window — so a single one-shot enrich still yields
// a real value (mirrors WakeupsCollector, since enrich runs on one sample).
const gpuWindow = 200 * time.Millisecond

// maxGPUScans bounds concurrent /proc/PID/fd scans, as FDCollector does, so a
// large process count cannot create an I/O storm (RFC §22).
const maxGPUScans = 32

// GPUCollector derives per-process GPU engine utilization and memory from the
// vendor-neutral DRM fdinfo ABI (`/proc/PID/fdinfo/<drm-fd>`, RFC §13). One
// collector covers Intel (i915/xe), AMD (amdgpu) and ARM DRM drivers — engine
// names are read, never assumed. Proprietary NVIDIA (which does not expose the
// standard fdinfo) is simply invisible here; a process with no DRM client leaves
// the metrics absent (renders "-"), never a fabricated zero.
type GPUCollector struct {
	root   string
	window time.Duration
	sleep  func(time.Duration) // injectable so tests drive the window deterministically
	sem    chan struct{}
}

// NewGPUCollector builds a collector rooted at the given procfs path (default
// /proc when empty).
func NewGPUCollector(root string) *GPUCollector {
	if root == "" {
		root = "/proc"
	}
	return &GPUCollector{root: root, window: gpuWindow, sleep: time.Sleep, sem: make(chan struct{}, maxGPUScans)}
}

// ID matches Descriptor.Collector for the gpu metrics.
func (c *GPUCollector) ID() string { return "gpu" }

// gpuEngineIDs are the canonical per-engine utilization metrics (PM-0511). DRM
// engine names are driver-defined, so each driver's names are mapped onto these
// standard classes (canonicalEngine); engines a driver does not expose stay 0
// only when the process holds a DRM client (else absent).
var gpuEngineIDs = []model.MetricID{"gpu-render", "gpu-compute", "gpu-copy", "gpu-video", "gpu-video-enhance"}

// Metrics lists the produced ids.
func (c *GPUCollector) Metrics() []model.MetricID {
	return append([]model.MetricID{"gpu", "gpu-mem"}, gpuEngineIDs...)
}

// canonicalEngine maps a driver-specific DRM engine name to a standard class, or
// "" to leave it only in the summed `gpu` total. Covers i915/xe (render/copy/
// video/video-enhance), amdgpu (gfx/compute/dma/dec/enc/jpeg) and common ARM
// names — read from fdinfo, never assumed present.
func canonicalEngine(name string) string {
	switch name {
	case "render", "gfx", "3d":
		return "gpu-render"
	case "compute":
		return "gpu-compute"
	case "copy", "dma", "blitter":
		return "gpu-copy"
	case "video", "dec", "enc", "bsd", "vcn", "jpeg":
		return "gpu-video"
	case "video-enhance", "vebox":
		return "gpu-video-enhance"
	default:
		return ""
	}
}

// gpuAgg is a process's GPU usage aggregated across its unique DRM clients.
type gpuAgg struct {
	busyNs   uint64             // summed engine busy nanoseconds across engines/clients
	engineNs map[string]uint64  // per canonical engine metric id -> busy nanoseconds
	memBytes uint64             // best-effort resident/used GPU memory
	hasDRM   bool               // the process holds at least one DRM client
	readErr  model.Availability // set when /proc/PID/fd could not be read
}

// Collect enriches each process with GPU utilization + memory. It reads once,
// waits gpuWindow, and reads the busy counters again; utilization is the delta
// over the window. When no process holds a DRM client there is nothing to time,
// so the window (and its sleep) is skipped entirely.
func (c *GPUCollector) Collect(ctx context.Context, procs []model.Process) {
	first := c.scanAll(ctx, procs)
	anyDRM := false
	for _, a := range first {
		if a.hasDRM {
			anyDRM = true
			break
		}
	}
	if !anyDRM {
		c.reportReadErrors(procs, first)
		return
	}

	c.sleep(c.window)
	windowNs := float64(c.window.Nanoseconds())
	for i := range procs {
		p := &procs[i]
		a1, ok := first[p.PID]
		switch {
		case !ok:
			// launch was cancelled before this process was scanned
		case a1.readErr != "":
			c.markUnavailable(p, a1.readErr)
		case !a1.hasDRM:
			// not a GPU client: leave both metrics absent (renders "-")
		default:
			c.applyWindow(p, a1, windowNs)
		}
	}
}

// applyWindow re-reads the process's DRM counters and sets gpu/gpu-mem from the
// delta since the first read.
func (c *GPUCollector) applyWindow(p *model.Process, a1 gpuAgg, windowNs float64) {
	a2, err := c.scanOne(p.PID)
	if err != nil {
		c.markUnavailable(p, classifyReadErr(err))
		return
	}
	// Memory is a gauge — report the latest read.
	p.SetMetric("gpu-mem", model.NewValue(float64(a2.memBytes), model.Sampled, "gpu"))
	if windowNs <= 0 || a2.busyNs < a1.busyNs {
		// counter reset / reused pid within the window: no trustworthy rate yet
		p.SetMetric("gpu", model.Unavailable[float64](model.WarmingUp, "gpu"))
		for _, id := range gpuEngineIDs {
			p.SetMetric(id, model.Unavailable[float64](model.WarmingUp, "gpu"))
		}
		return
	}
	p.SetMetric("gpu", model.NewValue(busyPct(a1.busyNs, a2.busyNs, windowNs), model.Sampled, "gpu"))
	for _, id := range gpuEngineIDs {
		p.SetMetric(id, model.NewValue(busyPct(a1.engineNs[string(id)], a2.engineNs[string(id)], windowNs), model.Sampled, "gpu"))
	}
}

// busyPct is the % of the window a counter of busy nanoseconds advanced (clamped
// at 0 on a counter reset).
func busyPct(ns1, ns2 uint64, windowNs float64) float64 {
	if ns2 < ns1 {
		return 0
	}
	return float64(ns2-ns1) / windowNs * 100
}

// reportReadErrors surfaces permission/read failures even when nothing turned out
// to hold a DRM client, so "couldn't check" never masquerades as "no GPU".
func (c *GPUCollector) reportReadErrors(procs []model.Process, first map[int]gpuAgg) {
	for i := range procs {
		if a, ok := first[procs[i].PID]; ok && a.readErr != "" {
			c.markUnavailable(&procs[i], a.readErr)
		}
	}
}

func (c *GPUCollector) markUnavailable(p *model.Process, reason model.Availability) {
	for _, id := range c.Metrics() {
		p.SetMetric(id, model.Unavailable[float64](reason, "gpu"))
	}
}

// scanAll reads every process's DRM usage concurrently (bounded), returning a
// per-pid aggregate; read failures are recorded on the aggregate.
func (c *GPUCollector) scanAll(ctx context.Context, procs []model.Process) map[int]gpuAgg {
	results := make([]gpuAgg, len(procs))
	errs := make([]error, len(procs))
	done := make(chan struct{})
	var launched int
	for i := range procs {
		if ctx.Err() != nil {
			break
		}
		launched++
		go func(i int) {
			c.sem <- struct{}{}
			defer func() { <-c.sem; done <- struct{}{} }()
			results[i], errs[i] = c.scanOne(procs[i].PID)
		}(i)
	}
	for i := 0; i < launched; i++ {
		<-done
	}
	out := make(map[int]gpuAgg, launched)
	for i := 0; i < launched; i++ {
		a := results[i]
		if errs[i] != nil {
			a.readErr = classifyReadErr(errs[i])
		}
		out[procs[i].PID] = a
	}
	return out
}

// scanOne aggregates a single process's DRM clients: it finds `/dev/dri/*` fds,
// parses their fdinfo, and sums engine busy-time + memory across *unique* clients
// (deduped by pdev+client-id, so dup'd fds are not double counted).
func (c *GPUCollector) scanOne(pid int) (gpuAgg, error) {
	var agg gpuAgg
	fdDir := filepath.Join(c.root, itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return agg, err
	}
	fdinfoDir := filepath.Join(c.root, itoa(pid), "fdinfo")
	seen := map[string]bool{}
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, e.Name()))
		if err != nil || !isDRMTarget(target) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fdinfoDir, e.Name()))
		if err != nil {
			continue // fd closed mid-scan
		}
		cl := parseDRMFdinfo(string(data))
		if !cl.isDRM {
			continue
		}
		agg.hasDRM = true
		if key := cl.dedupKey(); key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		agg.busyNs += cl.busyNs
		agg.memBytes += cl.memBytes()
		for id, ns := range cl.engineNs {
			if agg.engineNs == nil {
				agg.engineNs = map[string]uint64{}
			}
			agg.engineNs[id] += ns
		}
	}
	return agg, nil
}

// isDRMTarget reports whether an fd symlink target is a DRM device node
// (/dev/dri/card* or /dev/dri/renderD*), the fds that carry drm-* fdinfo.
func isDRMTarget(target string) bool { return strings.HasPrefix(target, "/dev/dri/") }

// drmClient is one parsed DRM fdinfo record.
type drmClient struct {
	isDRM    bool
	pdev     string
	clientID string
	busyNs   uint64            // total across engines
	engineNs map[string]uint64 // canonical engine metric id -> busy ns
	resident uint64
	memory   uint64
	total    uint64
}

// drmParse holds the transient per-engine data needed to apply the cycles-based
// fallback after a record is fully read.
type drmParse struct {
	nsEngines map[string]bool   // engines that reported drm-engine-<n> (ns) directly
	cycles    map[string]uint64 // engine -> drm-cycles-<n>
	maxfreq   map[string]uint64 // engine -> drm-maxfreq-<n> (Hz)
}

// memBytes picks the best available memory figure: resident, else the legacy
// drm-memory, else total allocated — different drivers emit different subsets.
func (d drmClient) memBytes() uint64 {
	switch {
	case d.resident > 0:
		return d.resident
	case d.memory > 0:
		return d.memory
	default:
		return d.total
	}
}

// dedupKey identifies a unique GPU client so dup'd fds are counted once; empty
// when the driver emits no client-id (then every fd is treated as distinct).
func (d drmClient) dedupKey() string {
	if d.clientID == "" {
		return ""
	}
	return d.pdev + "/" + d.clientID
}

// parseDRMFdinfo parses the `drm-*` keys of a `/proc/PID/fdinfo/<fd>` record. It
// is vendor-neutral: engine names are summed as they appear, memory regions are
// bucketed by resident/memory/total. Pure and unit-tested.
func parseDRMFdinfo(s string) drmClient {
	c := drmClient{engineNs: map[string]uint64{}}
	p := drmParse{nsEngines: map[string]bool{}, cycles: map[string]uint64{}, maxfreq: map[string]uint64{}}
	for _, line := range strings.Split(s, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		c.consume(strings.TrimSpace(key), strings.TrimSpace(val), &p)
	}
	c.applyCyclesFallback(&p)
	return c
}

// consume folds one `drm-*` fdinfo line into the client record.
func (c *drmClient) consume(key, val string, p *drmParse) {
	switch {
	case key == "drm-driver":
		c.isDRM = true
	case key == "drm-pdev":
		c.pdev = val
	case key == "drm-client-id":
		c.isDRM = true
		c.clientID = val
	case strings.HasPrefix(key, "drm-engine-capacity-"):
		// capacity is not busy time; ignore.
	case strings.HasPrefix(key, "drm-engine-"):
		c.isDRM = true
		name := strings.TrimPrefix(key, "drm-engine-")
		c.addEngine(name, parseFirstUint(val)) // "<ns> ns"
		p.nsEngines[name] = true
	case strings.HasPrefix(key, "drm-cycles-"):
		p.cycles[strings.TrimPrefix(key, "drm-cycles-")] += parseFirstUint(val)
	case strings.HasPrefix(key, "drm-maxfreq-"):
		p.maxfreq[strings.TrimPrefix(key, "drm-maxfreq-")] = parseFirstUint(val)
	default:
		c.consumeMem(key, val)
	}
}

func (c *drmClient) consumeMem(key, val string) {
	switch {
	case strings.HasPrefix(key, "drm-resident-"):
		c.resident += parseMemBytes(val)
	case strings.HasPrefix(key, "drm-memory-"):
		c.memory += parseMemBytes(val)
	case strings.HasPrefix(key, "drm-total-"):
		c.total += parseMemBytes(val)
	}
}

// addEngine accumulates busy nanoseconds into the total and the canonical engine.
func (c *drmClient) addEngine(name string, ns uint64) {
	c.busyNs += ns
	if b := canonicalEngine(name); b != "" {
		c.engineNs[b] += ns
	}
}

// applyCyclesFallback derives busy ns from drm-cycles/drm-maxfreq for engines
// that did not report drm-engine-<n> in nanoseconds (some ARM/v3d drivers).
func (c *drmClient) applyCyclesFallback(p *drmParse) {
	for name, cyc := range p.cycles {
		if p.nsEngines[name] {
			continue // ns already counted; don't double
		}
		hz := p.maxfreq[name]
		if hz == 0 {
			continue
		}
		c.addEngine(name, uint64(float64(cyc)/float64(hz)*1e9))
	}
}

// parseFirstUint reads the leading unsigned integer of a value like "12345 ns".
func parseFirstUint(val string) uint64 {
	fields := strings.Fields(val)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[0], 10, 64)
	return n
}

// parseMemBytes reads a memory value like "1024 KiB" / "512" into bytes.
func parseMemBytes(val string) uint64 {
	fields := strings.Fields(val)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseUint(fields[0], 10, 64)
	if len(fields) == 1 {
		return n
	}
	switch fields[1] {
	case "KiB":
		return n * 1024
	case "MiB":
		return n * 1024 * 1024
	case "GiB":
		return n * 1024 * 1024 * 1024
	default: // "B", bytes, or unknown → treat as bytes
		return n
	}
}

func classifyReadErr(err error) model.Availability {
	switch {
	case os.IsPermission(err):
		return model.PermissionDenied
	case os.IsNotExist(err):
		return model.Vanished
	default:
		return model.ReadError
	}
}
