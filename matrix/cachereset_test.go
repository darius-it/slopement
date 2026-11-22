package matrix

import "testing"

// Resetting the cache with none open still answers, and leaves no cache to
// write to.
func TestResetCacheWithoutOne(t *testing.T) {
	s, _ := testSynced()
	s.dirtyMeta = true
	done := false
	s.ResetCache(func() { done = true })
	if !done || s.cache != nil || s.dirtyMeta {
		t.Fatalf("done %v, cache %v, dirtyMeta %v", done, s.cache, s.dirtyMeta)
	}
}
