// Package history is the optional, opt-in audit log of control actions (RFC
// §16.1, §23). It records only non-sensitive fields (no cmdlines or paths) and
// is disabled by default. Events append to $XDG_STATE_HOME/procfit/events.ndjson.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is a single audit record. It deliberately carries no cmdline/path data.
type Event struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"` // applied|restored|drifted|reapplied|stopped|continued|signaled
	Target string    `json:"target"`
	PID    int       `json:"pid"`
	Field  string    `json:"field,omitempty"` // nice|stop
	From   string    `json:"from,omitempty"`
	To     string    `json:"to,omitempty"`
	Note   string    `json:"note,omitempty"`
}

// Recorder records audit events. Implementations must be safe for concurrent use.
type Recorder interface {
	Record(Event)
}

// Nop is a Recorder that discards events (history disabled).
type Nop struct{}

// Record does nothing.
func (Nop) Record(Event) {}

// FileRecorder appends events as newline-delimited JSON to a private file.
type FileRecorder struct {
	mu   sync.Mutex
	path string
}

// Open creates the history directory (0700) and returns a file recorder.
func Open(dir string) (*FileRecorder, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FileRecorder{path: filepath.Join(dir, "events.ndjson")}, nil
}

// Record appends one event (best-effort; audit must never break control flow).
func (r *FileRecorder) Record(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')
	r.mu.Lock()
	defer r.mu.Unlock()
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}
