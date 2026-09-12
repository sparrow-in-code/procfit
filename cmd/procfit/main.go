// Command procfit is a low-overhead Linux process observer and workload
// controller. This entry point is intentionally thin: it delegates all logic to
// internal/app so the behaviour is testable without a process boundary.
package main

import (
	"os"

	"github.com/netikras/procfit/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
