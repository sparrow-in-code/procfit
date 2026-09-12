// Package resolve decorates normalized entities with derived labels (user
// names, systemd units, app ids). Resolvers implement ports.Resolver and are
// applied as a chain; a failure leaves the entity untouched (RFC §14.2).
package resolve

import (
	"os"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/model"
)

// UserResolver maps UID to a login name using a passwd file, cached after the
// first read. The passwd path is configurable for testing.
type UserResolver struct {
	path   string
	cache  map[uint32]string
	loaded bool
}

// NewUserResolver builds a resolver reading the given passwd file (default
// /etc/passwd when empty).
func NewUserResolver(path string) *UserResolver {
	if path == "" {
		path = "/etc/passwd"
	}
	return &UserResolver{path: path, cache: map[uint32]string{}}
}

// ID identifies the resolver.
func (r *UserResolver) ID() string { return "user" }

// Resolve sets p.User from the uid, if a name is known.
func (r *UserResolver) Resolve(p *model.Process) {
	if !r.loaded {
		r.load()
	}
	if name, ok := r.cache[p.UID]; ok {
		p.User = name
	}
}

func (r *UserResolver) load() {
	r.loaded = true
	data, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	for uid, name := range parsePasswd(data) {
		r.cache[uid] = name
	}
}

// parsePasswd extracts uid->name mappings from passwd content. Malformed lines
// are skipped. Pure and testable.
func parsePasswd(data []byte) map[uint32]string {
	out := map[uint32]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		uid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		out[uint32(uid)] = fields[0]
	}
	return out
}
