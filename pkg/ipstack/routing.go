package ipstack

import (
	"log/slog"
	"net/netip"
	"time"
)

// Goroutine to send periodic RIP updates
func (s *IPStack) PeriodicUpdate(updateRate time.Duration) {
	// slog.Info("Starting periodic update", "updateRate", updateRate)
	ticker := time.NewTicker(updateRate)
	defer ticker.Stop()

	for {
		// Wait for ticker
		<-ticker.C

		// Send RIP Response to all neighbors
		for _, neighbor := range s.IPConfig.RipNeighbors {
			s.SendRIPResponse(neighbor, s.GetAllRIPEntries())
		}
	}
}

// Handle RIP Packets
func RIPHandler(packet *IPPacket, stack *IPStack) {
	// slog.Info("Received RIP packet", "source", packet.SourceIP, "destination", packet.DestinationIP, "protocol", packet.Protocol, "ttl", packet.TTL)
	ripMessage, err := UnmarshalRIPMessage(packet.Payload)
	if err != nil {
		slog.Error("Error unmarshalling RIP message", "error", err)
		return
	}

	switch ripMessage.command {
	case RIP_REQUEST:
		// slog.Info("Received RIP request")
		stack.SendRIPResponse(packet.SourceIP, stack.GetAllRIPEntries())
	case RIP_RESPONSE:
		// slog.Info("Received RIP response")
		stack.ProcessRIPResponse(packet.SourceIP, ripMessage)
	}
}

// Send RIP Request to all neighbors
func (s *IPStack) SendRIPRequest() {
	// slog.Info("Sending RIP request")
	message := RIPMessage{
		command:     RIP_REQUEST,
		num_entries: 0,
		entries:     []RIPMessageEntry{},
	}

	marshalled_message, err := MarshalRIPMessage(message)
	if err != nil {
		slog.Error("Error marshalling RIP message", "error", err)
		return
	}


	// slog.Info("Sending RIP request to neighbors", "neighbors", s.IPConfig.RipNeighbors)
	// Loop through forwarding table and send RIP request to all neighbors of RIP routes
	for _, neighbor := range s.IPConfig.RipNeighbors {
		// slog.Info("Sending RIP request to neighbor", "neighbor", neighbor)
		s.SendIP(neighbor, RIP_PROTOCOL, 1 + 1, marshalled_message)
	}
}


// Send RIP Response to destination
func (s *IPStack) SendRIPResponse(dst netip.Addr, entries []RIPMessageEntry) {
	response := RIPMessage{
		command:     RIP_RESPONSE,
		num_entries: uint16(len(entries)),
		entries:     entries,
	}

	marshalled_message, err := MarshalRIPMessage(response)
	if err != nil {
		slog.Error("Error marshalling RIP message", "error", err)
		return
	}

	err = s.SendIP(dst, RIP_PROTOCOL, 1 + 1, marshalled_message)
	if err != nil {
		slog.Error("Error sending RIP response", "error", err)
	}
}

// Process RIP Response
func (s *IPStack) ProcessRIPResponse(sourceIP netip.Addr, ripMessage RIPMessage) {
	changedEntries := make([]RIPMessageEntry, 0)

	// slog.Info("Processing RIP response", "num_entries", len(ripMessage.entries))

	for _, entry := range ripMessage.entries {
		destAddr := uint32ToNetipAddr(entry.address)
		destPrefix := netip.PrefixFrom(destAddr, int(entry.mask))
		cost := int(entry.cost) + 1

		// slog.Info("Processing RIP response", "destAddr", destAddr, "mask", entry.mask, "destPrefix", destPrefix, "cost", cost)

		if cost >= 16 {
			// TODO: Check this, should we remove or just not add
			// might need to keep this for poison reverse
			oldEntry, _ := s.ForwardingTable.Lookup(destPrefix)
			if oldEntry.NextHop == sourceIP {
				oldEntry.LastUpdated = time.Now()
				oldEntry.Metric = cost
			}

			continue
		} else {
			oldEntry, exists := s.ForwardingTable.Lookup(destPrefix)
			if !exists || cost < oldEntry.Metric {
				s.ForwardingTable.AddRoute(ForwardingTableEntry{
					DestinationPrefix: destPrefix,
					NextHop:           sourceIP,
					Interface:         s.getInterfaceForIP(sourceIP),
					Metric:            cost,
					Source:            SourceRIP,
					LastUpdated:       time.Now(),
				})
				changedEntries = append(changedEntries, RIPMessageEntry{
					address: entry.address,
					mask:    entry.mask,
					cost:    uint32(cost),
				})
			} else if oldEntry.NextHop == sourceIP {
				// slog.Info("Same route update received", "destPrefix", destPrefix, "cost", cost, "source", sourceIP)
				// Update timestamp and cost, check if something else should be updated
				oldEntry.LastUpdated = time.Now()
				oldEntry.Metric = cost
			}
		}
	}

	if len(changedEntries) > 0 {
		s.SendTriggeredUpdate(changedEntries)
	}
}

// Send triggered update to all neighbors
func (s *IPStack) SendTriggeredUpdate(changedEntries []RIPMessageEntry) {
	for _, neighbor := range s.IPConfig.RipNeighbors {
		poisonedEntries := s.applyPoisonReverse(changedEntries, neighbor)
		s.SendRIPResponse(neighbor, poisonedEntries)
	}
}

// Apply poison reverse and split horizon
func (s *IPStack) applyPoisonReverse(entries []RIPMessageEntry, neighbor netip.Addr) []RIPMessageEntry {
	poisonedEntries := make([]RIPMessageEntry, 0, len(entries))
	for _, entry := range entries {
		destPrefix := netip.PrefixFrom(uint32ToNetipAddr(entry.address), int(entry.mask))
		route, exists := s.ForwardingTable.Lookup(destPrefix)
		if exists && route.NextHop == neighbor {
			// Poison reverse
			poisonedEntries = append(poisonedEntries, RIPMessageEntry{
				address: entry.address,
				mask:    entry.mask,
				cost:    16, // Infinity
			})
		} else {
			// Split horizon
			if route.NextHop != neighbor {
				poisonedEntries = append(poisonedEntries, entry)
			}
		}
	}
	return poisonedEntries
}

// Get the interface name for a given IP
func (s *IPStack) getInterfaceForIP(ip netip.Addr) string {
	for name, iface := range s.Interfaces {
		if iface.Netmask.Contains(ip) {
			return name
		}
	}
	return ""
}

func (s *IPStack) GetAllRIPEntries() []RIPMessageEntry {
	entries := make([]RIPMessageEntry, 0)
	for _, entry := range s.ForwardingTable.Entries {
		addr := entry.DestinationPrefix.Addr().As4()
		ipUint32 := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])
		entries = append(entries, RIPMessageEntry{
			address: ipUint32,
			mask:    uint32(entry.DestinationPrefix.Bits()),
			cost:    uint32(entry.Metric),
		})
	}
	return entries
}

// Convert uint32 to netip.Addr
func uint32ToNetipAddr(ipUint32 uint32) netip.Addr {
	ipBytes := [4]byte{
		byte(ipUint32 >> 24),
		byte(ipUint32 >> 16),
		byte(ipUint32 >> 8),
		byte(ipUint32),
	}
	return netip.AddrFrom4(ipBytes)
}

// Function to run RIP timeout check on Go routine
func (s *IPStack) RIPTimeoutCheck(timeout time.Duration) {
	// slog.Info("Starting RIP timeout check", "timeout", timeout)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		// Wait for ticker
		<-ticker.C

		// Check for timeouts
		for _, entry := range s.ForwardingTable.Entries {
			if entry.Source == SourceRIP {
				if time.Since(entry.LastUpdated) > timeout {
					s.ForwardingTable.RemoveRoute(entry.DestinationPrefix)
				}
			}
		}
	}
}
