package matrix

import (
	"context"
	"syscall"
)

// netChanges tells, on the channel, that the network changed: an interface came
// or went, an address or a route changed. It reads the kernel's routing socket,
// which blocks (no timer, no CPU) until the system has something to say. The
// channel closes when ctx ends or the socket fails.
func netChanges(ctx context.Context) <-chan struct{} {
	fd, err := syscall.Socket(syscall.AF_ROUTE, syscall.SOCK_RAW, 0)
	if err != nil {
		return nil
	}
	out := make(chan struct{}, 1)
	go func() {
		<-ctx.Done()
		syscall.Close(fd) // unblocks the read below
	}()
	go func() {
		defer close(out)
		buf := make([]byte, 2048)
		for {
			n, err := syscall.Read(fd, buf)
			if err != nil {
				return
			}
			// A routing message: length (2 bytes), version, type.
			if n < 4 {
				continue
			}
			switch int(buf[3]) {
			case syscall.RTM_IFINFO, syscall.RTM_NEWADDR, syscall.RTM_DELADDR, syscall.RTM_ADD, syscall.RTM_DELETE:
				select {
				case out <- struct{}{}:
				default: // one is already waiting to be read
				}
			}
		}
	}()
	return out
}
