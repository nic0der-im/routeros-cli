// Package session provides safe-mode style change journals with rollback.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	StatusActive    = "active"
	StatusCommitted = "committed"
	StatusRolledBack = "rolled_back"
)

// Change records a single mutating operation and how to undo it.
type Change struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	Inverse   []string  `json:"inverse"` // first element is command, rest are args
	AppliedAt time.Time `json:"applied_at"`
	Note      string    `json:"note,omitempty"`
}

// Session is a journal of pending changes for a device.
type Session struct {
	ID        string    `json:"id"`
	Device    string    `json:"device"`
	Safe      bool      `json:"safe"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Changes   []Change  `json:"changes"`
}

// Store persists sessions to disk under a base directory.
type Store struct {
	mu  sync.Mutex
	dir string
}

// NewStore creates a session store rooted at dir (created if needed).
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating session dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// DefaultDir returns ~/.config/ros/sessions.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".config", "ros", "sessions")
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) activePath(device string) string {
	return filepath.Join(s.dir, "active-"+sanitize(device)+".lock")
}

func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// Begin starts a new active session for a device. Only one active session per device.
func (s *Store) Begin(device string, safe bool) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.activePath(device)); err == nil {
		return nil, fmt.Errorf("device %q already has an active session; commit or rollback first", device)
	}

	now := time.Now().UTC()
	sess := &Session{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		Device:    device,
		Safe:      safe,
		Status:    StatusActive,
		StartedAt: now,
		UpdatedAt: now,
		Changes:   []Change{},
	}

	if err := s.write(sess); err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.activePath(device), []byte(sess.ID), 0o600); err != nil {
		_ = os.Remove(s.path(sess.ID))
		return nil, fmt.Errorf("writing active lock: %w", err)
	}
	return sess, nil
}

// Active returns the active session for a device, if any.
func (s *Store) Active(device string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.activePath(device))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return s.readUnlocked(string(data))
}

// Get loads a session by ID.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnlocked(id)
}

func (s *Store) readUnlocked(id string) (*Session, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("reading session %q: %w", id, err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parsing session %q: %w", id, err)
	}
	return &sess, nil
}

func (s *Store) write(sess *Session) error {
	sess.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(sess.ID), data, 0o600)
}

// AppendChange records a change on an active session.
func (s *Store) AppendChange(sess *Session, change Change) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.Status != StatusActive {
		return fmt.Errorf("session %q is not active", sess.ID)
	}
	if change.ID == "" {
		change.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if change.AppliedAt.IsZero() {
		change.AppliedAt = time.Now().UTC()
	}
	sess.Changes = append(sess.Changes, change)
	return s.write(sess)
}

// Commit marks the session as committed and clears the active lock.
func (s *Store) Commit(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.Status != StatusActive {
		return fmt.Errorf("session %q is not active", sess.ID)
	}
	sess.Status = StatusCommitted
	if err := s.write(sess); err != nil {
		return err
	}
	_ = os.Remove(s.activePath(sess.Device))
	return nil
}

// MarkRolledBack marks the session rolled back and clears the active lock.
func (s *Store) MarkRolledBack(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess.Status = StatusRolledBack
	if err := s.write(sess); err != nil {
		return err
	}
	_ = os.Remove(s.activePath(sess.Device))
	return nil
}

// BuildInverse constructs a simple inverse for common RouterOS mutations.
// Returns command + args for the inverse, or empty if unknown.
func BuildInverse(command string, args []string, createdID string) []string {
	switch {
	case endsWith(command, "/add") && createdID != "":
		base := command[:len(command)-len("/add")]
		return []string{base + "/remove", "=.id=" + createdID}
	case endsWith(command, "/enable") && createdID != "":
		base := command[:len(command)-len("/enable")]
		return []string{base + "/disable", "=.id=" + createdID}
	case endsWith(command, "/disable") && createdID != "":
		base := command[:len(command)-len("/disable")]
		return []string{base + "/enable", "=.id=" + createdID}
	default:
		return nil
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
