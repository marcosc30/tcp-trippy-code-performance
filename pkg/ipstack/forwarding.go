package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"net/netip"
	"sync"
	"ip-rip-in-peace/pkg/lnxconfig"
)

// This function validates a pakcet by checking TTL and checksum
func ValidatePacket(packet IPPacket) bool {
	if packet.TTL <= 0 {
		return false
	}

	checksum := packet.CalculateChecksum()
	if checksum != packet.Checksum {
		return false
	}

	return true
}

func RecievePacket(packet *IPPacket, ipstack *IPStack) {
	// Recieve packet at the network level

	// 1. Validate packet
	if !ValidatePacket(*packet) {
		// Drop packet
		// TODO: Log error
		return
	}

	next_hop := NextHop(packet.DestinationIP, ipstack.ForwardingTable)
	// 2. Is it for me?



}

func ForwardPacket(packet *IPPacket, forward_table *ForwardingTable) {
	// 
}

// Uses longest-prefix matching to find the next hop for a destination
func NextHop(destination netip.Addr, forwardingTable *ForwardingTable) netip.Addr {
	forwardingTable.Mutex.Lock();
	defer forwardingTable.Mutex.Unlock();

	bestMatch := netip.Addr{}
	bestPrefix := netip.Prefix{}

	for _, entry := range forwardingTable.Entries {
		if entry.DestinationPrefix.Contains(destination) {
			if entry.DestinationPrefix.Len() > bestPrefix.Len() {
				bestPrefix = entry.DestinationPrefix
				bestMatch = entry.NextHop
			}
		}
	}


	return bestMatch
}

type IPStack struct {
	Interfaces      map[string]*Interface
	ForwardingTable *ForwardingTable
	// Maybe a handler function as well for routers sending RIP updates?
	Mutex           sync.RWMutex // Protects shared resources
	// IPConfig 	  *lnxconfig.IPConfig // We add this in case we need to access some information like TCP or router timing parameters
	Handlers      map[uint8]HandlerFunc
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


type HandlerFunc func(...) 

func (s *IPStack) Initialize(configInfo IPConfig) (error) {
	s.Handlers = make(map[uint8]HandlerFunc)

	return IPStack{}, nil
}

func (s *IPStack) SendIP(dst netip.Addr, protocolNum uint8, data []byte) error {
	return nil
}

func (s *IPStack) RegisterHandler(protocolNum uint8, handler HandlerFunc) {
	s.Handlers[protocolNum] = handler
}

func (s *IPStack) HandlePacket(packet IPPacket) {
	
}
