package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"net/netip"
	"sync"
	"time"
)

type ForwardingTableEntry struct {
	DestinationPrefix netip.Prefix
	NextHop           netip.Addr
	Interface         string // Interface identifier (e.g., "if0")
	Metric            int
	Source            RouteSource
	LastUpdated       time.Time // Last time this route was updated
}

type RouteSource string

const (
	SourceStatic    RouteSource = "STATIC"
	SourceRIP       RouteSource = "RIP"
	SourceLocal     RouteSource = "LOCAL"
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

// Returns true if the route was added, false if it was updated
// Adds if route is not present or cost is less than existing route
func (ft *ForwardingTable) AddRoute(entry ForwardingTableEntry) {
	ft.Mutex.Lock()
	defer ft.Mutex.Unlock()

	// First, check if the route is already in the table
	for i, e := range ft.Entries {
		if e.DestinationPrefix == entry.DestinationPrefix {
			if entry.Metric < e.Metric {
				ft.Entries[i] = entry
			}
			return
		}
	}

	// If not found, add it
	ft.Entries = append(ft.Entries, entry)
}

// Returns the route entry for a given prefix if it exists
func (ft *ForwardingTable) Lookup(prefix netip.Prefix) (*ForwardingTableEntry, bool) {
	ft.Mutex.RLock()
	defer ft.Mutex.RUnlock()

	for _, e := range ft.Entries {
		if e.DestinationPrefix == prefix {
			return &e, true
		}
	}
	return &ForwardingTableEntry{}, false
}

// Removes a route from the forwarding table
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
