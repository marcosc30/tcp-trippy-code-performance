package ipstack

// This is the main file that contains the forwarding logic, as well as important structs for IP like IPStack, and forwarding table

import (
	"log/slog"
	"net/netip"
	"sync"
)

// This function validates a pakcet by checking TTL and checksum
func ValidatePacket(packet IPPacket) bool {
	if packet.TTL <= 0 {
		slog.Info("Invalid TTL")
		return false
	}

	checksum := packet.CalculateChecksum()
	if checksum != packet.Checksum {
		slog.Info("Invalid checksum")
		return false
	}
	return true
}

func ReceivePacket(packet *IPPacket, ipstack *IPStack) {
	// Print packet info

	// Receive packet at the network level

	// 1. Validate packet
	if !ValidatePacket(*packet) {
		// Drop packet
		return
	}

	// 2. For me? Check all interfaces
	for _, iface := range ipstack.Interfaces {
		if iface.IPAddr == packet.DestinationIP {
			// Packet is for me
			// Handle packet
			ipstack.HandlePacket(packet)
		}
	}

	// 3. Forward packet, doesn't matter if it is local or not based on
	// how we have it structured
	interfaceName, nextHop := NextHop(packet.DestinationIP, ipstack.ForwardingTable)

	// Check for no match
	if interfaceName == "" {
		slog.Info("No match in forwarding table")
		return
	}

	nextIF := ipstack.Interfaces[interfaceName]

	// Decrement TTL
	packet.TTL--

	// Recalculate checksum
	packet.Checksum = packet.CalculateChecksum()

	// Send packet to next hop
	nextIF.SendPacket(packet, nextHop)
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

type IPStack struct {
	Interfaces      map[string]*Interface
	ForwardingTable *ForwardingTable
	// Maybe a handler function as well for routers sending RIP updates?
	Mutex sync.RWMutex // Protects shared resources
	// IPConfig 	  *lnxconfig.IPConfig // We add this in case we need to access some information like TCP or router timing parameters
	Handlers map[uint8]HandlerFunc
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

type HandlerFunc func(*IPPacket)

func (s *IPStack) SendIP(dst netip.Addr, protocolNum uint8, ttl uint8, data []byte) error {
	// We treat it the same as receive packet, but we don't need to decrement TTL

	// We increment TTL by one to counter the decrement in ReceivePacket
	packet, err := CreatePacket(s.Interfaces["if0"].IPAddr.String(), dst.String(), ttl, protocolNum, string(data))
	if err != nil {
		return err
	}

	ReceivePacket(&packet, s)

	return nil
}

func (s *IPStack) RegisterHandler(protocolNum uint8, handler HandlerFunc) {
	s.Handlers[protocolNum] = handler
}

func (s *IPStack) HandlePacket(packet *IPPacket) {
	// Check if we have a handler for this protocol
	handler, ok := s.Handlers[packet.Protocol]
	if !ok {
		// Drop packet
		return
	}

	handler(packet)
}
