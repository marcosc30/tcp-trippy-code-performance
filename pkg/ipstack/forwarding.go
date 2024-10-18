package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"net/netip"
	"sync"
)

type ForwardingTableEntry struct {
	DestinationPrefix netip.Prefix
	NextHop           netip.Addr
	Interface         string // Interface identifier (e.g., "if0")
	Metric            int
	Source            RouteSource
}

type RouteSource string

const (
	SourceStatic    RouteSource = "STATIC"
	SourceRIP       RouteSource = "RIP"
	SourceConnected RouteSource = "CONNECTED"
)

type ForwardingTable struct {
	Entries []ForwardingTableEntry
	Mutex   sync.RWMutex
}

// Uses longest-prefix matching to find the next hop for a destination
func (ft *ForwardingTable) NextHop(destination netip.Addr) (string, netip.Addr) {
	ft.Mutex.RLock()
	defer ft.Mutex.RUnlock()

	var bestMatch ForwardingTableEntry
	var bestPrefix netip.Prefix

	for _, entry := range ft.Entries {
		if entry.DestinationPrefix.Contains(destination) {
			if bestPrefix.Bits() == 0 || entry.DestinationPrefix.Bits() > bestPrefix.Bits() {
				bestMatch = entry
				bestPrefix = entry.DestinationPrefix
			}
		}
	}

	return bestMatch.Interface, bestMatch.NextHop
}

func (ft *ForwardingTable) AddRoute(entry ForwardingTableEntry) {
	ft.Mutex.Lock()
	defer ft.Mutex.Unlock()

	for i, e := range ft.Entries {
		if e.DestinationPrefix == entry.DestinationPrefix {
			ft.Entries[i] = entry
			return
		}
	}
	ft.Entries = append(ft.Entries, entry)
}

func (ft *ForwardingTable) RemoveRoute(prefix netip.Prefix) {
	ft.Mutex.Lock()
	defer ft.Mutex.Unlock()

	for i, e := range ft.Entries {
		if e.DestinationPrefix == prefix {
			ft.Entries = append(ft.Entries[:i], ft.Entries[i+1:]...)
			return
		}
	}
}
