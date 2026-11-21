package matrix

import (
	"sync"

	"github.com/egoist/mygo"
)

// memSecrets is a keychain in memory, with the real one's error contract.
type memSecrets struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memSecrets) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = append([]byte(nil), value...)
	return nil
}

func (s *memSecrets) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.m[key]; ok {
		return append([]byte(nil), b...), nil
	}
	return nil, mygo.ErrSecretNotFound
}

func (s *memSecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

// Isolate points the package at a directory of its own and a keychain in
// memory, for the tests of packages that sign in: without it a test would
// record its fake account among the user's real ones, and its tokens in
// their real keychain. restore puts the real ones back.
func Isolate(dir string) (restore func()) {
	oldDir, oldSecrets := configDir, secrets
	configDir = func() (string, error) { return dir, nil }
	secrets = &memSecrets{m: map[string][]byte{}}
	return func() { configDir, secrets = oldDir, oldSecrets }
}
