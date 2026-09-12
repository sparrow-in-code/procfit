// Package control is the OS-agnostic control side plane (RFC §6.2, §15): it
// resolves managed targets to live instances, revalidates identity before acting,
// applies nice/stop intent, and records original/desired/observed state for safe
// restore and drift detection. It depends on the Controller and ProcessSource
// ports, never on syscalls directly.
package control

import (
	"time"

	"github.com/netikras/procfit/internal/model"
)

// FieldStatus is the lifecycle status of a controlled field or an apply result
// per instance (RFC §15.2, §21.4).
type FieldStatus string

const (
	StatusApplied     FieldStatus = "applied"
	StatusUnchanged   FieldStatus = "unchanged"
	StatusDrifted     FieldStatus = "drifted"
	StatusRestored    FieldStatus = "restored"
	StatusUnavailable FieldStatus = "unavailable"
	StatusVanished    FieldStatus = "vanished"
	StatusStale       FieldStatus = "stale"
	StatusDenied      FieldStatus = "permission_denied"
	StatusFailed      FieldStatus = "failed"
	StatusSkipped     FieldStatus = "skipped_protected"
)

// NiceField holds the original/desired/observed nice values for one instance
// (RFC §15.2). Original is captured exactly once, before the first mutation.
type NiceField struct {
	Original   int         `json:"original"`
	Desired    int         `json:"desired"`
	Observed   int         `json:"observed"`
	Status     FieldStatus `json:"status"`
	Backend    string      `json:"backend"`
	CapturedAt time.Time   `json:"captured_at"`
	ChangedAt  time.Time   `json:"changed_at"`
	ObservedAt time.Time   `json:"observed_at"`
	Error      string      `json:"error,omitempty"`
}

// StopField tracks procfit's own SIGSTOP intent, kept separate from the observed
// kernel state (RFC §15.4).
type StopField struct {
	DesiredStop bool        `json:"desired_stop"`
	PreStopped  bool        `json:"pre_stopped"`
	Status      FieldStatus `json:"status"`
	ChangedAt   time.Time   `json:"changed_at"`
	Error       string      `json:"error,omitempty"`
}

// Binding is a concrete instance bound to a target, with its captured control
// fields.
type Binding struct {
	ID   model.ProcessInstanceID `json:"id"`
	PID  int                     `json:"pid"`
	Nice *NiceField              `json:"nice,omitempty"`
	Stop *StopField              `json:"stop,omitempty"`
}

// BindingMode is follow (re-resolve continuously) or snapshot (fixed set).
type BindingMode string

const (
	ModeFollow   BindingMode = "follow"
	ModeSnapshot BindingMode = "snapshot"
)

// Origin distinguishes config policy from runtime-manual targets.
type Origin string

const (
	OriginRuntime Origin = "runtime_manual"
	OriginConfig  Origin = "config_policy"
)

// Target is a persistent logical target that binds to zero or more instances
// (RFC §11.4). It remains visible when inactive (RFC §16.6).
type Target struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Origin      Origin                    `json:"origin"`
	BindingMode BindingMode               `json:"binding_mode"`
	Selector    string                    `json:"selector,omitempty"`
	SnapshotIDs []model.ProcessInstanceID `json:"snapshot_ids,omitempty"`

	DesiredNice *int   `json:"desired_nice,omitempty"`
	DesiredStop *bool  `json:"desired_stop,omitempty"`
	OnDrift     string `json:"on_drift,omitempty"` // report|reapply|adopt|ignore

	Bindings []Binding `json:"bindings"`
	LastSeen time.Time `json:"last_seen"`
	Active   bool      `json:"active"`
}

// State is the persisted runtime state (RFC §16.3). It is boot-scoped: a boot id
// mismatch discards concrete bindings.
type State struct {
	SchemaVersion int       `json:"schema_version"`
	BootID        string    `json:"boot_id"`
	OwnerUID      int       `json:"owner_uid"`
	Generation    int       `json:"generation"`
	UpdatedAt     time.Time `json:"updated_at"`
	Targets       []Target  `json:"targets"`
}

// SchemaVersion is the current runtime-state schema version.
const SchemaVersion = 1
