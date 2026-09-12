package app

import (
	"flag"
	"strings"

	"github.com/netikras/procfit/internal/queryspec"
)

// queryFlags holds the shared observation flags (RFC §8.1) parsed for ps/stat.
type queryFlags struct {
	groupBy     string
	leaf        string
	sortSpec    string
	columns     string
	profile     string
	metricOver  stringList
	format      string
	selectExpr  string
	havingExpr  string
	number      int
	human       bool
	targetWidth int
	configPath  string
	noConfig    bool
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
	fs.IntVar(&qf.number, "number", 0, "show only the top N rows after sorting (0 = all)")
	fs.IntVar(&qf.number, "n", 0, "alias for --number")
	fs.BoolVar(&qf.human, "human", false, "human-readable units (K/M/G); default prints raw bytes/numbers")
	fs.BoolVar(&qf.human, "h", false, "alias for --human")
	fs.IntVar(&qf.targetWidth, "target-width", 0, "cap the TARGET column width (0 = auto-size)")
	fs.StringVar(&qf.configPath, "config", "", "config file to load (defaults: $PROCFIT_CONFIG or XDG)")
	fs.BoolVar(&qf.noConfig, "no-config", false, "ignore any config file")
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
		Number:          qf.number,
		Human:           qf.human,
		TargetWidth:     qf.targetWidth,
	}
}

// resolveQuery compiles the flags into a query via the shared queryspec package.
func (a *assembly) resolveQuery(qf *queryFlags) (queryspec.Resolved, error) {
	return queryspec.Build(a.reg, a.dims, qf.toSpecFlags())
}
