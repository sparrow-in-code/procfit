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
	configPrec  string
	showConfig  bool
	sources     map[string]Source // resolved layer per setting (for --show-config)
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
	fs.StringVar(&qf.groupBy, "group-by", "", "group by dimensions (comma-separated for sub-groups); "+
		"values: none,host,comm,name,user,exe,app,cgroup,systemd-unit,container,pod,uid,namespace-set "+
		"(default: none = flat process list)")
	fs.StringVar(&qf.leaf, "leaf", "", "terminal rows under groups; values: process,thread,none (default: process)")
	fs.StringVar(&qf.sortSpec, "sort", "", "sort keys 'field[:asc|desc]', comma-separated, e.g. cpu:desc,comm (default: unsorted)")
	fs.StringVar(&qf.columns, "columns", "", "explicit columns, comma-separated; ids from 'procfit metrics' plus "+
		"target,pid,tid,pt,user,pstate,... (default: auto for the leaf mode)")
	fs.StringVar(&qf.profile, "metrics", "", "metric profile; values: none,light,process,io,network,power,battery,perf,all (default: light)")
	fs.Var(&qf.metricOver, "metric", "add/remove one metric: NAME, +NAME, or -NAME (repeatable; ids from 'procfit metrics')")
	fs.StringVar(&qf.format, "format", "table", "output format; values: table,wide,json,ndjson,csv")
	fs.StringVar(&qf.selectExpr, "select", "", "pre-group entity filter expression, e.g. 'uid == 0 && cpu > 5'")
	fs.StringVar(&qf.havingExpr, "having", "", "post-group row filter expression, e.g. 'rss > 100M || cpu > 20'")
	fs.IntVar(&qf.number, "number", 0, "show only the top N rows after sorting (default: 0 = all)")
	fs.IntVar(&qf.number, "n", 0, "alias for --number")
	fs.BoolVar(&qf.human, "human", false, "human-readable units K/M/G (default: raw bytes/numbers)")
	fs.BoolVar(&qf.human, "h", false, "alias for --human")
	fs.IntVar(&qf.targetWidth, "target-width", 0, "cap the TARGET column width in chars (default: 0 = auto-size)")
	fs.StringVar(&qf.configPath, "config", "", "config file to load (defaults: $PROCFIT_CONFIG or XDG)")
	fs.BoolVar(&qf.noConfig, "no-config", false, "ignore any config file")
	fs.StringVar(&qf.configPrec, "config-precedence", "", "layer order low→high (default: config,env,args); 'default' is always the floor")
	fs.BoolVar(&qf.showConfig, "show-config", false, "print the effective settings with their source, then exit")
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
