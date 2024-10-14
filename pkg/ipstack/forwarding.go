package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"net/netip"
	"sync"
)

// This function validates a pakcet by checking TTL and checksum
func ValidatePacket(packet string) bool {
	return true
}

func ForwardPacket(packet string, forward_table ForwardingTable) {
	// Forward packet to next hop
}

// Uses longest-prefix matching to find the next hop for a destination
func NextHop(destination string, forward_table map[string]string) string {
	// Get next hop for destination

	return ""
}

type IPStack struct {
	Interfaces      map[string]*Interface
	ForwardingTable *ForwardingTable
	// Maybe a handler function as well for routers sending RIP updates?
	Mutex           sync.RWMutex // Protects shared resources
}

type ForwardingTableEntry struct {
	DestinationPrefix netip.Prefix
	NextHop           netip.Addr
	Interface         string // Interface identifier (e.g., "if0")
}

type ForwardingTable struct {
	Entries []ForwardingTableEntry
	Mutex   sync.RWMutex
}
