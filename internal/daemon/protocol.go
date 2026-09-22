// Package daemon implements the optional per-user engine daemon and its IPC. The
// daemon owns the authoritative runtime state (single writer, RFC §16.4) and
// runs one shared collector for all clients (RFC §17). The wire protocol is a
// versioned, newline-delimited JSON request/response over an AF_UNIX socket with
// peer-credential authentication (RFC §17.3).
package daemon

import "encoding/json"

// ProtocolVersion is the IPC protocol version negotiated in Hello.
const ProtocolVersion = 1

// MaxMessageBytes bounds a single request/response frame (RFC §17.3).
const MaxMessageBytes = 4 << 20 // 4 MiB

// RequestType enumerates client request kinds.
type RequestType string

const (
	ReqHello   RequestType = "hello"
	ReqQuery   RequestType = "query"
	ReqManaged RequestType = "managed"
	ReqControl RequestType = "control"
	ReqHealth  RequestType = "health"
	ReqReload  RequestType = "reload"
)

// Request is a single client request.
type Request struct {
	Version int         `json:"version"`
	Type    RequestType `json:"type"`
	Query   *QueryReq   `json:"query,omitempty"`
	Control *ControlReq `json:"control,omitempty"`
}

// QueryReq describes an observation query using raw (serializable) fields; the
// daemon compiles selectors/predicates on its side (the compiled QuerySpec is
// not serializable).
type QueryReq struct {
	GroupBy string `json:"group_by,omitempty"`
	Leaf    string `json:"leaf,omitempty"`
	Sort    string `json:"sort,omitempty"`
	Columns string `json:"columns,omitempty"`
	Metrics string `json:"metrics,omitempty"`
	Select  string `json:"select,omitempty"`
	Having  string `json:"having,omitempty"`
}

// ControlReq describes a control operation.
type ControlReq struct {
	Op     string `json:"op"` // set-nice|restore|unmanage|stop|continue|signal
	Target string `json:"target"`
	Nice   *int   `json:"nice,omitempty"`
	Force  bool   `json:"force,omitempty"`
	Signal string `json:"signal,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// Response is the daemon's reply.
type Response struct {
	Version int             `json:"version"`
	Error   string          `json:"error,omitempty"`
	Hello   *HelloResp      `json:"hello,omitempty"`
	Rows    json.RawMessage `json:"rows,omitempty"`    // JSON query envelope
	Managed json.RawMessage `json:"managed,omitempty"` // managed target listing
	Control json.RawMessage `json:"control,omitempty"` // control result
	Health  *HealthResp     `json:"health,omitempty"`
}

// HelloResp reports the daemon's version and capabilities.
type HelloResp struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	NiceCtl   bool   `json:"nice_control"`
	SignalCtl bool   `json:"signal_control"`
}

// HealthResp reports engine health (RFC §23).
type HealthResp struct {
	UptimeSeconds  float64 `json:"uptime_seconds"`
	Generation     int     `json:"generation"`
	Clients        int     `json:"clients"`
	LastScanMillis float64 `json:"last_scan_millis"`
	Processes      int     `json:"processes"`
}
