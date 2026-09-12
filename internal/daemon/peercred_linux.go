//go:build linux

package daemon

import (
	"net"
	"syscall"
)

// peerUID returns the connecting peer's UID via SO_PEERCRED, so the daemon can
// reject other users (RFC §17.3).
func peerUID(conn net.Conn) (int, bool) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, false
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, false
	}
	var uid int
	var found bool
	_ = raw.Control(func(fd uintptr) {
		cred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if err == nil {
			uid = int(cred.Uid)
			found = true
		}
	})
	return uid, found
}
