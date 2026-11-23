//go:build !darwin && !linux

package matrix

import "context"

// netChanges has no system event to wait on here (Windows, phones): nil, and the
// sync's own failures raise the offline bar. No polling instead.
func netChanges(ctx context.Context) <-chan struct{} { return nil }
