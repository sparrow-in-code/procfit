package model

import (
	"fmt"
	"sort"
	"strings"
)

// NamespaceType enumerates the Linux namespace kinds (RFC §11.2).
type NamespaceType string

const (
	NSPID    NamespaceType = "pid"
	NSMount  NamespaceType = "mnt"
	NSNet    NamespaceType = "net"
	NSUser   NamespaceType = "user"
	NSUTS    NamespaceType = "uts"
	NSIPC    NamespaceType = "ipc"
	NSCgroup NamespaceType = "cgroup"
	NSTime   NamespaceType = "time"
)

// AllNamespaceTypes lists the namespace kinds in a stable order.
var AllNamespaceTypes = []NamespaceType{NSPID, NSMount, NSNet, NSUser, NSUTS, NSIPC, NSCgroup, NSTime}

// NamespaceID identifies a namespace by kind and inode. The inode is the native
// identity; friendly names are resolver metadata only and must never replace it
// (RFC §11.2).
type NamespaceID struct {
	Type  NamespaceType
	Inode uint64
}

// String renders e.g. "pid:4026531836".
func (n NamespaceID) String() string {
	return fmt.Sprintf("%s:%d", n.Type, n.Inode)
}

// NamespaceSet is the tuple of a process's namespace memberships. Grouping by
// namespace-set groups processes sharing the whole isolation boundary (RFC
// §11.2).
type NamespaceSet map[NamespaceType]uint64

// Get returns the inode for a namespace type and whether it is known.
func (s NamespaceSet) Get(t NamespaceType) (uint64, bool) {
	if s == nil {
		return 0, false
	}
	v, ok := s[t]
	return v, ok
}

// Key returns a stable, order-independent key for the whole set, suitable for
// grouping by namespace-set.
func (s NamespaceSet) Key() string {
	if len(s) == 0 {
		return ""
	}
	types := make([]string, 0, len(s))
	for t := range s {
		types = append(types, string(t))
	}
	sort.Strings(types)
	var b strings.Builder
	for i, t := range types {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s:%d", t, s[NamespaceType(t)])
	}
	return b.String()
}
