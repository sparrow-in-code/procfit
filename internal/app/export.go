package app

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/netikras/procfit/internal/meta"
	"github.com/netikras/procfit/internal/queryspec"
	promrender "github.com/netikras/procfit/internal/render/prometheus"
)

// cmdExport implements `procfit export`: render the current sample in Prometheus
// text format, either once to stdout or as a scrapable /metrics HTTP endpoint
// (PM-9006). It reuses the full assembly (all collectors), and the warm-up sample
// keeps rate metrics meaningful on each scrape.
func cmdExport(env Env, args []string) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	qf := bindQueryFlags(fs)
	listen := fs.String("listen", "", "serve Prometheus /metrics on this address (e.g. :9256); empty = one-shot to stdout")
	instant := fs.Bool("instant", false, "skip the warm-up sample; rate metrics render unavailable")
	setupUsage(env, fs, "export", "export metrics in Prometheus text format (one-shot or /metrics server)",
		"procfit export --profile network                 # one-shot to stdout (cron/textfile)",
		"procfit export --profile network --group-by comm --leaf none --listen :9256   # scrape target",
		"GET /metrics?profile=io&group_by=comm&leaf=none  # per-scrape query overrides")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	a, err := newAssemblyFn()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if *listen == "" {
		r, err := a.resolveQuery(qf)
		if err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitUsage
		}
		if err := a.exportRender(context.Background(), r, *instant, env.Stdout); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
		return ExitOK
	}
	return a.exportServe(env, *listen, qf, *instant)
}

// exportRender samples once (with the warm-up unless instant), builds, and writes
// Prometheus text.
func (a *assembly) exportRender(ctx context.Context, r queryspec.Resolved, instant bool, w io.Writer) error {
	in, err := a.sampleForResult(ctx, r, instant)
	if err != nil {
		return err
	}
	res, err := a.engine.Build(in, r.Spec)
	if err != nil {
		return err
	}
	return promrender.Render(w, res, a.reg)
}

// exportServe runs the /metrics HTTP endpoint; each scrape re-resolves the query
// with any URL overrides so different jobs can pull different views.
func (a *assembly) exportServe(env Env, addr string, base *queryFlags, instant bool) int {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		qf := exportOverrides(*base, req.URL.Query())
		r, err := a.resolveQuery(&qf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Render into a buffer so query/sample errors surface before any bytes are
		// written (and a client disconnect mid-write can't double-write the header).
		var buf bytes.Buffer
		if err := a.exportRender(req.Context(), r, instant, &buf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	})
	fmt.Fprintf(env.Stdout, "%s exporting Prometheus metrics on %s/metrics\n", meta.Name, addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

// exportOverrides returns a copy of the base flags with the query view overridden
// by scrape URL params (profile/group_by/leaf/select/having), so a scrape can pick
// its own view without a restart.
func exportOverrides(qf queryFlags, q url.Values) queryFlags {
	set := func(dst *string, keys ...string) {
		for _, k := range keys {
			if v := q.Get(k); v != "" {
				*dst = v
				return
			}
		}
	}
	set(&qf.profile, "profile", "metrics")
	set(&qf.groupBy, "group_by", "group-by")
	set(&qf.leaf, "leaf")
	set(&qf.selectExpr, "select")
	set(&qf.havingExpr, "having")
	return qf
}
