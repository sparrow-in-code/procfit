package app

import (
	"flag"
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// queryFlags holds the shared observation flags (RFC §8.1) parsed for ps/stat.
type queryFlags struct {
	groupBy    string
	leaf       string
	sortSpec   string
	columns    string
	profile    string
	metricOver stringList
	format     string
	selectExpr string
	havingExpr string
}

// stringList is a repeatable string flag (e.g. --metric).
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func bindQueryFlags(fs *flag.FlagSet) *queryFlags {
	qf := &queryFlags{}
	fs.StringVar(&qf.groupBy, "group-by", "", "ordered grouping dimensions, comma-separated (or 'none')")
	fs.StringVar(&qf.leaf, "leaf", "", "leaf mode: process|thread|none")
	fs.StringVar(&qf.sortSpec, "sort", "", "sort keys, e.g. cpu:desc,comm")
	fs.StringVar(&qf.columns, "columns", "", "explicit columns, comma-separated")
	fs.StringVar(&qf.profile, "metrics", "", "metric profile (none|light|process|io|...)")
	fs.Var(&qf.metricOver, "metric", "add/remove a metric: name, +name, or -name (repeatable)")
	fs.StringVar(&qf.format, "format", "table", "output: table|wide|json|ndjson|csv")
	fs.StringVar(&qf.selectExpr, "select", "", "entity selector expression")
	fs.StringVar(&qf.havingExpr, "having", "", "aggregate/row filter expression")
	return qf
}

// resolved holds everything needed to run and render a query.
type resolved struct {
	spec    query.QuerySpec
	needed  []model.MetricID
	columns []string
	format  string
}

var defaultColumns = []string{"target", "pt", "cpu", "rss", "disk-rbps", "disk-wbps", "pstate"}

// wideColumns is the expanded default column set for --format wide (RFC §18).
var wideColumns = []string{
	"target", "pt", "user", "pid", "cpu", "cpu-user", "cpu-system",
	"rss", "vsz", "minor-faults", "major-faults",
	"disk-rbps", "disk-wbps", "pstate", "pnice",
}

func (a *assembly) resolveQuery(qf *queryFlags) (resolved, error) {
	var r resolved
	r.format = qf.format

	profile := qf.profile
	if profile == "" {
		profile = string(metrics.ProfileLight)
	}
	overrides, err := parseOverrides(qf.metricOver)
	if err != nil {
		return r, err
	}
	selection, err := a.reg.ResolveSelection(metrics.ProfileName(profile), overrides)
	if err != nil {
		return r, err
	}

	cols := defaultColumns
	if qf.format == "wide" {
		cols = wideColumns
	}
	if qf.columns != "" {
		cols = splitComma(qf.columns)
	}
	r.columns = cols

	sortKeys, err := parseSort(qf.sortSpec)
	if err != nil {
		return r, err
	}

	groupBy := splitComma(qf.groupBy)
	leaf := query.LeafMode(qf.leaf)
	if leaf == "" {
		leaf = query.LeafProcess
	}

	selPred, havPred, exprFields, err := a.compilePredicates(qf)
	if err != nil {
		return r, err
	}

	needed := a.neededMetrics(selection, append(cols, exprFields...), sortKeys)
	r.needed = needed
	r.spec = query.QuerySpec{
		Metrics: needed,
		Select:  selPred,
		GroupBy: groupBy,
		Leaf:    leaf,
		Having:  havPred,
		Sort:    sortKeys,
		Columns: cols,
	}
	return r, nil
}

// compilePredicates compiles and validates the optional --select/--having
// expressions and returns them plus the referenced fields (so needed metrics can
// include them).
func (a *assembly) compilePredicates(qf *queryFlags) (query.EntityPredicate, query.RowPredicate, []string, error) {
	var selPred query.EntityPredicate
	var havPred query.RowPredicate
	var fields []string
	if qf.selectExpr != "" {
		prog, err := compileValidated(qf.selectExpr, a.entityAllowedFields())
		if err != nil {
			return nil, nil, nil, err
		}
		selPred = selectPred{prog: prog}
		fields = append(fields, prog.Fields()...)
	}
	if qf.havingExpr != "" {
		prog, err := compileValidated(qf.havingExpr, a.rowAllowedFields())
		if err != nil {
			return nil, nil, nil, err
		}
		havPred = havingPred{prog: prog}
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

// neededMetrics is the union of the profile selection plus any metric referenced
// by a column or sort key, so nothing a view needs is left uncomputed.
func (a *assembly) neededMetrics(selection []model.MetricID, cols []string, sort []query.SortKey) []model.MetricID {
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
		if id, ok := a.reg.ResolveID(c); ok {
			add(id)
		}
	}
	for _, k := range sort {
		if id, ok := a.reg.ResolveID(k.Field); ok {
			add(id)
		}
	}
	return order
}

func parseOverrides(raw []string) ([]metrics.Override, error) {
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
	for _, part := range splitComma(spec) {
		k, err := parseSortKey(part)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func parseSortKey(part string) (query.SortKey, error) {
	// Forms: field, field:asc, field:desc, -field, +field.
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

func splitComma(s string) []string {
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
