// Package state persists the control plane's runtime state privately and
// atomically (RFC §16). The directory is 0700 and files 0600; writes go through a
// temp file + rename so a crash never leaves a half-written state; a corrupt file
// is quarantined rather than silently overwritten.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/netikras/procfit/internal/control"
)

// Store owns the runtime state directory.
type Store struct {
	dir string
}

const stateFile = "state.json"

// Open ensures the directory exists with safe permissions and returns a Store.
// It refuses a directory owned by another user is left to the caller's env; here
// we create it 0700 if absent.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("state: empty directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("state: mkdir: %w", err)
	}
	// Tighten perms in case the dir pre-existed with looser mode.
	_ = os.Chmod(dir, 0o700)
	return &Store{dir: dir}, nil
}

func (s *Store) path() string { return filepath.Join(s.dir, stateFile) }

// Load reads the persisted state. It returns ok=false when no state exists yet.
// A corrupt file is moved aside (.corrupt) and treated as absent so the user can
// inspect it (RFC §16.3).
func (s *Store) Load() (*control.State, bool, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var st control.State
	if err := json.Unmarshal(data, &st); err != nil {
		_ = os.Rename(s.path(), s.path()+".corrupt")
		return nil, false, fmt.Errorf("state: corrupt file quarantined: %w", err)
	}
	return &st, true, nil
}

// Save atomically writes the state: temp file → fsync → rename → dir fsync.
func (s *Store) Save(st *control.State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path()); err != nil {
		return err
	}
	return fsyncDir(s.dir)
}

func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	// Best-effort: some filesystems don't support directory fsync.
	_ = d.Sync()
	return nil
}
