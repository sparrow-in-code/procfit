package ports

import "github.com/netikras/procfit/internal/model"

// Resolver decorates a normalized entity with derived labels/identity (user
// names, systemd unit, app id, …) without changing native keys (RFC §14.2). A
// resolver that fails must leave the base entity intact (no error is surfaced;
// it simply adds nothing). Resolvers compose as a chain (Decorator pattern).
type Resolver interface {
	ID() string
	Resolve(p *model.Process)
}
