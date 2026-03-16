package credential

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

const serviceName = "routeros-cli"

// ErrNotFound is returned when a credential does not exist in the store.
var ErrNotFound = errors.New("credential not found")

// Store provides access to stored router credentials.
type Store interface {
	Get(deviceName string) (password string, err error)
	Set(deviceName, password string) error
	Delete(deviceName string) error
}

// KeyringStore implements Store using the system keyring via go-keyring.
type KeyringStore struct{}

// NewKeyringStore returns a KeyringStore backed by the OS credential manager.
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{}
}

func (s *KeyringStore) Get(deviceName string) (string, error) {
	password, err := keyring.Get(serviceName, deviceName)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return password, nil
}

func (s *KeyringStore) Set(deviceName, password string) error {
	return keyring.Set(serviceName, deviceName, password)
}

func (s *KeyringStore) Delete(deviceName string) error {
	err := keyring.Delete(serviceName, deviceName)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// MemoryStore implements Store using an in-memory map. Intended for testing.
type MemoryStore struct {
	mu    sync.RWMutex
	creds map[string]string
}

// NewMemoryStore returns a MemoryStore ready for use.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		creds: make(map[string]string),
	}
}

func (s *MemoryStore) Get(deviceName string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	password, ok := s.creds[deviceName]
	if !ok {
		return "", ErrNotFound
	}
	return password, nil
}

func (s *MemoryStore) Set(deviceName, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.creds[deviceName] = password
	return nil
}

func (s *MemoryStore) Delete(deviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.creds[deviceName]; !ok {
		return ErrNotFound
	}
	delete(s.creds, deviceName)
	return nil
}
