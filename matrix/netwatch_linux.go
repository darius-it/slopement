package matrix

import (
	"context"
	"syscall"
)

// The kernel's netlink multicast groups (linux/rtnetlink.h; Go's syscall package
// does not name them).
const (
	rtmgrpLink      = 0x1
	rtmgrpIPv4Addr  = 0x10
	rtmgrpIPv4Route = 0x40
	rtmgrpIPv6Addr  = 0x100
)

// netChanges tells, on the channel, that the network changed: a link came or
// went, or an address or route changed. It reads a netlink socket subscribed to
// those, which blocks (no timer, no CPU) until the kernel has something to say.
// The channel closes when ctx ends or the socket fails.
func netChanges(ctx context.Context) <-chan struct{} {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil
	}
	err = syscall.Bind(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: rtmgrpLink | rtmgrpIPv4Addr | rtmgrpIPv4Route | rtmgrpIPv6Addr,
	})
	if err != nil {
		syscall.Close(fd)
		return nil
	}
	out := make(chan struct{}, 1)
	go func() {
		<-ctx.Done()
		syscall.Shutdown(fd, syscall.SHUT_RDWR) // unblocks the read below
		syscall.Close(fd)
	}()
	go func() {
		defer close(out)
		buf := make([]byte, 4096)
		for {
			if _, err := syscall.Read(fd, buf); err != nil {
				return
			}
			select {
			case out <- struct{}{}:
			default: // one is already waiting to be read
			}
		}
	}()
	return out
}
