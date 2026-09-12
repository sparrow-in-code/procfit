package app

import (
	"io"
	"os"
)

// isTerminal reports whether w is an interactive terminal. It avoids an external
// dependency by inspecting the file mode for a character device.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
