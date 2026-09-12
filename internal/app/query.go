package app

import (
	"flag"
	"strings"

	"github.com/netikras/procfit/internal/queryspec"
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

func (qf *queryFlags) toSpecFlags() queryspec.Flags {
	return queryspec.Flags{
		GroupBy:         qf.groupBy,
		Leaf:            qf.leaf,
		Sort:            qf.sortSpec,
		Columns:         qf.columns,
		Profile:         qf.profile,
		MetricOverrides: qf.metricOver,
		Format:          qf.format,
		Select:          qf.selectExpr,
		Having:          qf.havingExpr,
	}
}

// resolveQuery compiles the flags into a query via the shared queryspec package.
func (a *assembly) resolveQuery(qf *queryFlags) (queryspec.Resolved, error) {
	return queryspec.Build(a.reg, a.dims, qf.toSpecFlags())
}
