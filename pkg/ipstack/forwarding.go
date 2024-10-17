package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"log/slog"
	"net/netip"
	"sync"
)

type ForwardingTableEntry struct {
	DestinationPrefix netip.Prefix
	NextHop           netip.Addr
	Interface         string // Interface identifier (e.g., "if0")
}

type ForwardingTable struct {
	Entries []ForwardingTableEntry
	Mutex   sync.RWMutex
}

// Uses longest-prefix matching to find the next hop for a destination
func NextHop(destination netip.Addr, forwardingTable *ForwardingTable) (string, netip.Addr) {
	forwardingTable.Mutex.Lock()
	defer forwardingTable.Mutex.Unlock()

	bestMatch := ForwardingTableEntry{}
	bestPrefix := netip.Prefix{}

	for _, entry := range forwardingTable.Entries {
		if entry.DestinationPrefix.Contains(destination) {
			if entry.DestinationPrefix.Bits() >= bestPrefix.Bits() {
				bestMatch = entry
				bestPrefix = entry.DestinationPrefix
			}
		}
	}

	return bestMatch.Interface, bestMatch.NextHop
}

