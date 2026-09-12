package app

import (
	"fmt"

	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
	"github.com/netikras/procfit/internal/render/csv"
	jsonrender "github.com/netikras/procfit/internal/render/json"
	"github.com/netikras/procfit/internal/render/ndjson"
	"github.com/netikras/procfit/internal/render/table"
)

// renderResult writes a query result in the requested format. Renderers are pure
// presentation (RFC §6.1); this only selects one.
func (a *assembly) renderResult(env Env, res *query.Result, format string, cols []string) int {
	switch format {
	case "", "table", "wide":
		return a.renderTable(env, res, cols)
	case "csv":
		return a.renderCSV(env, res, cols)
	case "json":
		if err := jsonrender.Render(env.Stdout, res, a.reg); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
	case "ndjson":
		if err := ndjson.NewStreamer(env.Stdout, a.reg).WriteResult(res); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
	default:
		fmt.Fprintf(env.Stderr, "unsupported format %q\n", format)
		return ExitUsage
	}
	return ExitOK
}

func (a *assembly) renderTable(env Env, res *query.Result, cols []string) int {
	resolved, err := render.ResolveColumns(a.reg, cols)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	if err := table.Render(env.Stdout, res, resolved); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

func (a *assembly) renderCSV(env Env, res *query.Result, cols []string) int {
	resolved, err := render.ResolveColumns(a.reg, cols)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	if err := csv.Render(env.Stdout, res, resolved); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}
