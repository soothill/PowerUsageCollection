package zeroconf

import (
	"context"
	"net"
)

// ServiceEntry represents a discovered service instance.
type ServiceEntry struct {
	Instance string
	HostName string
	Text     []string
	AddrIPv4 []net.IP
	AddrIPv6 []net.IP
}

// Resolver performs service browsing. This is a lightweight stub that
// immediately closes the provided results channel when the context is done.
type Resolver struct{}

// NewResolver returns a stub resolver. It intentionally ignores the
// provided configuration to keep the dependency offline-friendly.
func NewResolver(_ interface{}) (*Resolver, error) {
	return &Resolver{}, nil
}

// Browse starts a background goroutine that closes the entries channel once
// the context is done. No network discovery is performed in this stub
// implementation.
func (r *Resolver) Browse(ctx context.Context, _ string, _ string, entries chan<- *ServiceEntry) error {
	go func() {
		<-ctx.Done()
		close(entries)
	}()
	return nil
}
