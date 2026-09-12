//go:build !linux

package daemon

import "net"

// peerUID is unsupported off Linux; the daemon rejects connections it cannot
// authenticate.
func peerUID(_ net.Conn) (int, bool) { return 0, false }
