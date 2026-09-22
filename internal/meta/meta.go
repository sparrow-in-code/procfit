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
	// Version is a SemVer string with a build-metadata identifier, stamped at
	// build time as "<core>+<yyyyMMdd-HHmmss>-<sha>[.dirty]" (see
	// scripts/version.sh / flake.nix). The default marks an un-stamped dev build.
	Version = "0.1.0-dev"
	// Commit is the short git commit hash.
	Commit = "unknown"
	// BuildDate is the UTC build time as yyyyMMdd-HHmmss.
	BuildDate = "unknown"
)
