package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"sync/atomic"

	"github.com/netikras/procfit/internal/meta"
	jsonrender "github.com/netikras/procfit/internal/render/json"
)

// Server serves the IPC protocol over an AF_UNIX socket. It authenticates by
// peer credentials (only the owning UID) and owns the single runtime-state
// writer via the engine's manager (RFC §16.4, §17.3).
type Server struct {
	engine   *Engine
	saver    func() error
	ownerUID int
	clients  int32
}

// NewServer builds a server. saver persists runtime state after control ops.
func NewServer(engine *Engine, saver func() error, ownerUID int) *Server {
	return &Server{engine: engine, saver: saver, ownerUID: ownerUID}
}

// Listen creates the unix socket listener with user-only permissions, removing
// any stale socket first.
func Listen(sockPath string) (net.Listener, error) {
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(sockPath, 0o600)
	return ln, nil
}

// Serve accepts connections until the context is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if uid, ok := peerUID(conn); !ok || uid != s.ownerUID {
		_ = writeResponse(conn, Response{Version: ProtocolVersion, Error: "unauthorized: peer uid mismatch"})
		return
	}
	atomic.AddInt32(&s.clients, 1)
	defer atomic.AddInt32(&s.clients, -1)

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), MaxMessageBytes)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = writeResponse(conn, Response{Version: ProtocolVersion, Error: "bad request: " + err.Error()})
			continue
		}
		if err := writeResponse(conn, s.dispatch(req)); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(req Request) Response {
	switch req.Type {
	case ReqHello:
		return s.hello()
	case ReqQuery:
		return s.query(req.Query)
	case ReqManaged:
		return s.managed()
	case ReqControl:
		return s.control(req.Control)
	case ReqHealth:
		return Response{Version: ProtocolVersion, Health: ptrHealth(s.engine.Health(int(atomic.LoadInt32(&s.clients))))}
	default:
		return Response{Version: ProtocolVersion, Error: "unknown request type"}
	}
}

func (s *Server) hello() Response {
	caps := s.engine.ctrl.Capabilities()
	return Response{Version: ProtocolVersion, Hello: &HelloResp{
		Version: ProtocolVersion, Name: meta.Name, NiceCtl: caps.Nice, SignalCtl: caps.Signal,
	}}
}

func (s *Server) query(q *QueryReq) Response {
	if q == nil {
		return Response{Version: ProtocolVersion, Error: "query: missing payload"}
	}
	res, _, err := s.engine.RunQuery(reqToFlags(q))
	if err != nil {
		return Response{Version: ProtocolVersion, Error: err.Error()}
	}
	var buf bytes.Buffer
	if err := jsonrender.Render(&buf, res, s.engine.reg); err != nil {
		return Response{Version: ProtocolVersion, Error: err.Error()}
	}
	return Response{Version: ProtocolVersion, Rows: buf.Bytes()}
}

func (s *Server) managed() Response {
	data, err := json.Marshal(s.engine.mgr.State().Targets)
	if err != nil {
		return Response{Version: ProtocolVersion, Error: err.Error()}
	}
	return Response{Version: ProtocolVersion, Managed: data}
}

func writeResponse(conn net.Conn, resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = conn.Write(data)
	return err
}

func ptrHealth(h HealthResp) *HealthResp { return &h }
