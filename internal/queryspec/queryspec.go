// Package queryspec builds a renderer-agnostic query.QuerySpec from raw string
// flags (group-by, leaf, sort, columns, metric profile/overrides, select,
// having). It is shared by the CLI, the daemon, and the TUI so query semantics
// are identical everywhere (DRY; the "correct abstraction" that keeps those
// entry points thin). It depends only on the metric/dimension registries and the
// expr DSL.
package queryspec

import (
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// Flags is the raw, serializable description of a query.
type Flags struct {
	GroupBy         string
	Leaf            string
	Sort            string
	Columns         string
	Profile         string
	MetricOverrides []string
	Format          string
	Select          string
	Having          string
	// Number caps the top-level rows shown after sorting (0 = unlimited).
	Number int
	// Human enables unit-scaled output (K/M/G); off means raw bytes/numbers.
	Human bool
	// TargetWidth caps the TARGET column width (0 = auto-size to content).
	TargetWidth int
	// CollapseGroups starts the TUI with every group folded shut (view-only;
	// the query pipeline ignores it). New groups appearing later fold too.
	CollapseGroups bool
}

// Resolved is a compiled query plus the metrics it needs and the columns to
// render.
type Resolved struct {
	Spec        query.QuerySpec
	Needed      []model.MetricID
	Columns     []string
	Format      string
	Human       bool
	TargetWidth int
}

// DefaultColumns is the compact default column set. The leaf identifier (pid)
// is its own column so a single process is never confused with an aggregate
// group; thread-leaf views additionally get a tid column (see chooseColumns).
var DefaultColumns = []string{"target", "pid", "pt", "cpu", "rss", "disk-rbps", "disk-wbps", "pstate"}

// WideColumns is the expanded column set for --format wide (RFC §18).
var WideColumns = []string{
	"target", "pt", "user", "pid", "cpu", "cpu-user", "cpu-system",
	"rss", "vsz", "minor-faults", "major-faults",
	"disk-rbps", "disk-wbps", "pstate", "pnice",
}

// Build compiles the flags into a Resolved query against the given registries.
func Build(reg *metrics.Registry, dims *query.Dimensions, f Flags) (Resolved, error) {
	var r Resolved
	r.Format = f.Format
	r.Human = f.Human
	r.TargetWidth = f.TargetWidth

	selection, err := resolveSelection(reg, f)
	if err != nil {
		return r, err
	}
	cols := chooseColumns(reg, f)
	r.Columns = cols

	sortKeys, err := parseSort(f.Sort)
	if err != nil {
		return r, err
	}
	leaf := query.LeafMode(f.Leaf)
	if leaf == "" {
		leaf = query.LeafProcess
	}
	selPred, havPred, exprFields, err := compilePredicates(reg, dims, f)
	if err != nil {
		return r, err
	}
	needed := neededMetrics(reg, selection, append(cols, exprFields...), sortKeys)
	r.Needed = needed
	r.Spec = query.QuerySpec{
		Metrics: needed,
		Select:  selPred,
		GroupBy: SplitComma(f.GroupBy),
		Leaf:    leaf,
		Having:  havPred,
		Sort:    sortKeys,
		Columns: cols,
		Limit:   f.Number,
	}
	return r, nil
}

func resolveSelection(reg *metrics.Registry, f Flags) ([]model.MetricID, error) {
	profile := f.Profile
	if profile == "" {
		profile = string(metrics.ProfileLight)
	}
	overrides, err := ParseOverrides(f.MetricOverrides)
	if err != nil {
		return nil, err
	}
	return reg.ResolveSelection(metrics.ProfileName(profile), overrides)
}

func chooseColumns(reg *metrics.Registry, f Flags) []string {
	if f.Columns != "" {
		return SplitComma(f.Columns)
	}
	if f.Format == "wide" {
		return WideColumns
	}
	// An explicit metric profile drives the default display, so e.g.
	// `--metrics battery` actually shows the battery metrics (identity + members).
	if cols, ok := profileColumns(reg, f.Profile); ok {
		return leafIdentifiers(f.Leaf, cols)
	}
	if f.Leaf == string(query.LeafThread) {
		return withTID(DefaultColumns) // thread views expose the thread id too
	}
	return DefaultColumns
}

// profileColumns returns an explicit non-default profile's metric ids as columns
// (ok=false for the default/derived profiles, which keep the compact defaults).
func profileColumns(reg *metrics.Registry, profile string) ([]string, bool) {
	switch profile {
	case "", "none", "light", "all":
		return nil, false
	}
	ids, err := reg.Resolve(metrics.ProfileName(profile))
	if err != nil || len(ids) == 0 {
		return nil, false
	}
	cols := make([]string, 0, len(ids))
	for _, id := range ids {
		cols = append(cols, string(id))
	}
	return cols, true
}

// leafIdentifiers prepends the identity columns (target, pid, and tid for thread
// views) to a set of metric columns.
func leafIdentifiers(leaf string, metricCols []string) []string {
	head := []string{"target", "pid"}
	if leaf == string(query.LeafThread) {
		head = append(head, "tid")
	}
	return append(head, metricCols...)
}

// withTID returns cols with a "tid" column inserted right after "pid".
func withTID(cols []string) []string {
	out := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		out = append(out, c)
		if c == "pid" {
			out = append(out, "tid")
		}
	}
	return out
}

func compilePredicates(reg *metrics.Registry, dims *query.Dimensions, f Flags) (query.EntityPredicate, query.RowPredicate, []string, error) {
	var selPred query.EntityPredicate
	var havPred query.RowPredicate
	var fields []string
	if f.Select != "" {
		prog, err := compileValidated(f.Select, AllowedEntityFields(reg))
		if err != nil {
			return nil, nil, nil, err
		}
		selPred = SelectPred{Prog: prog}
		fields = append(fields, prog.Fields()...)
	}
	if f.Having != "" {
		prog, err := compileValidated(f.Having, AllowedRowFields(reg))
		if err != nil {
			return nil, nil, nil, err
		}
		havPred = HavingPred{Prog: prog}
		fields = append(fields, prog.Fields()...)
	}
	return selPred, havPred, fields, nil
}

func compileValidated(src string, allowed map[string]bool) (*expr.Program, error) {
	prog, err := expr.Compile(src)
	if err != nil {
		return nil, err
	}
	if err := prog.Validate(allowed); err != nil {
		return nil, err
	}
	return prog, nil
}

func neededMetrics(reg *metrics.Registry, selection []model.MetricID, cols []string, sort []query.SortKey) []model.MetricID {
	set := map[model.MetricID]bool{}
	order := []model.MetricID{}
	add := func(id model.MetricID) {
		if !set[id] {
			set[id] = true
			order = append(order, id)
		}
	}
	for _, id := range selection {
		add(id)
	}
	for _, c := range cols {
		if id, ok := reg.ResolveID(c); ok {
			add(id)
		}
	}
	for _, k := range sort {
		if id, ok := reg.ResolveID(k.Field); ok {
			add(id)
		}
	}
	return order
}

// ParseOverrides parses +name/-name/name metric overrides (RFC §8.2).
func ParseOverrides(raw []string) ([]metrics.Override, error) {
	var out []metrics.Override
	for _, r := range raw {
		if r == "" {
			continue
		}
		switch r[0] {
		case '+':
			out = append(out, metrics.Override{Add: true, ID: r[1:]})
		case '-':
			out = append(out, metrics.Override{Add: false, ID: r[1:]})
		default:
			out = append(out, metrics.Override{Add: true, ID: r})
		}
	}
	return out, nil
}

func parseSort(spec string) ([]query.SortKey, error) {
	if spec == "" {
		return nil, nil
	}
	var keys []query.SortKey
	for _, part := range SplitComma(spec) {
		k, err := parseSortKey(part)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func parseSortKey(part string) (query.SortKey, error) {
	if field, dir, ok := strings.Cut(part, ":"); ok {
		switch dir {
		case "asc":
			return query.SortKey{Field: field}, nil
		case "desc":
			return query.SortKey{Field: field, Descending: true}, nil
		default:
			return query.SortKey{}, fmt.Errorf("bad sort direction %q in %q", dir, part)
		}
	}
	switch {
	case strings.HasPrefix(part, "-"):
		return query.SortKey{Field: part[1:], Descending: true}, nil
	case strings.HasPrefix(part, "+"):
		return query.SortKey{Field: part[1:]}, nil
	default:
		return query.SortKey{Field: part}, nil
	}
}

// SplitComma splits and trims a comma-separated list, dropping empties.
func SplitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
