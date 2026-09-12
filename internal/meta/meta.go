// Package meta holds build/identity constants for the binary.
//
// The command name is deliberately kept in one place so it stays trivial to
// change (RFC §13, §30.1): the working name "procfit" may collide with an
// existing executable and could be rebranded without touching call sites.
package meta

// Name is the user-facing command/binary name. Change it here only.
const Name = "procfit"

// Description is a one-line summary used in help output.
const Description = "Linux process observer and workload controller"

// Build metadata, injected via -ldflags at build time (see Makefile).
var (
	// Version is the release/version string (e.g. a git tag).
	Version = "dev"
	// Commit is the short git commit hash.
	Commit = "none"
	// BuildDate is an RFC3339 UTC timestamp.
	BuildDate = "unknown"
)
