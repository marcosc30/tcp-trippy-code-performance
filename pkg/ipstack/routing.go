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
		// slog.Info("Sending periodic RIP update")

		// Send RIP Response to all neighbors
		for _, iface := range s.Interfaces {
			for neighbor := range iface.Neighbors {
				s.SendRIPResponse(neighbor, s.GetAllRIPEntries())
			}
		}
	}
}

// Handle RIP Packets
func RIPHandler(packet *IPPacket, stack *IPStack) {
	// slog.Info("Received RIP packet")
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
		command: RIP_REQUEST,
		num_entries: 0,
		entries: []RIPMessageEntry{},
	}

	marshalled_message, err := MarshalRIPMessage(message)
	if err != nil {
		slog.Error("Error marshalling RIP message", "error", err)
		return
	}

	for _, iface := range s.Interfaces {
		for neighbor := range iface.Neighbors {
			packet := IPPacket{
				SourceIP: iface.IPAddr,
				DestinationIP: neighbor,
				TTL: 2, // 1 + 1 for recieving protocol based on how we do it
				Protocol: RIP_PROTOCOL,
				Payload: marshalled_message,
			}
			iface.SendPacket(&packet, neighbor)
		}
	}
}


func (s *IPStack) SendRIPResponse(dst netip.Addr, entries []RIPMessageEntry) {
	response := RIPMessage{
		command:     RIP_RESPONSE,
		num_entries: uint16(len(entries)),
		entries:     entries,
	}

	// slog.Info("Sending RIP response", "dst", dst)
	// for _, entry := range entries {
	// 	slog.Info("\tEntry", "destAddr", uint32ToNetipAddr(entry.address), "mask", entry.mask, "cost", entry.cost)
	// }

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

// func HandleRIPPacket(packet *IPPacket, stack *IPStack) {
// 	ripMessage, err := UnmarshalRIPMessage(packet.Payload)
// 	if err != nil {
// 		slog.Error("Error unmarshalling RIP message", "error", err)
// 		return
// 	}

// 	switch ripMessage.command {
// 	case 1: // Request
// 		stack.SendRIPResponse(packet.SourceIP, stack.RIPTable.GetAllEntries())
// 	case 2: // Response
// 		stack.ProcessRIPResponse(packet.SourceIP, ripMessage)
// 	}
// }

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

			// Dont add
			continue

			// Remove the route if it exists
			// // Remove the route if it exists
			// s.ForwardingTable.RemoveRoute(destPrefix)
			// changedEntries = append(changedEntries, RIPMessageEntry{
			// 	address: entry.address,
			// 	mask:    entry.mask,
			// 	cost:    16, // Infinity
			// })
		} else {
			// slog.Info("", "destPrefix", destPrefix, "cost", cost)
			// Add or update route
			// TODO: Update if higher but from same neighbor
			oldEntry, exists := s.ForwardingTable.Lookup(destPrefix)
			if !exists || cost < oldEntry.Metric {
				// slog.Info("Better route found", "destPrefix", destPrefix, "cost", cost, "source", sourceIP)
				s.ForwardingTable.AddRoute(ForwardingTableEntry{
					DestinationPrefix: destPrefix,
					NextHop:           sourceIP,
					Interface:         s.getInterfaceForIP(sourceIP),
					Metric:            cost,
					Source:            SourceRIP,
				})
				changedEntries = append(changedEntries, RIPMessageEntry{
					address: entry.address,
					mask:    entry.mask,
					cost:    uint32(cost),
				})
			}
		}
	}

	if len(changedEntries) > 0 {
		s.SendTriggeredUpdate(changedEntries)
	}
}

func (s *IPStack) SendTriggeredUpdate(changedEntries []RIPMessageEntry) {
	// slog.Info("Sending triggered RIP update")
	for _, iface := range s.Interfaces {
		if iface.Down {
			continue
		}
		for neighbor := range iface.Neighbors {
			poisonedEntries := s.applyPoisonReverse(changedEntries, neighbor)
			s.SendRIPResponse(neighbor, poisonedEntries)
		}
	}
}

func (s *IPStack) applyPoisonReverse(entries []RIPMessageEntry, neighbor netip.Addr) []RIPMessageEntry {
	poisonedEntries := make([]RIPMessageEntry, 0, len(entries))
	for _, entry := range entries {
		destPrefix := netip.PrefixFrom(uint32ToNetipAddr(entry.address), int(entry.mask))
		route, exists := s.ForwardingTable.Lookup(destPrefix)
		if exists && route.NextHop == neighbor {
			// Apply poison reverse
			poisonedEntries = append(poisonedEntries, RIPMessageEntry{
				address: entry.address,
				mask:    entry.mask,
				cost:    16, // Infinity
			})
		} else {
			// Apply split horizon (don't send routes learned from this neighbor back to it)
			if route.NextHop != neighbor {
				poisonedEntries = append(poisonedEntries, entry)
			}
		}
	}
	return poisonedEntries
}


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


// TODO: Add RIP timeout threshold

// This function can run on a go routine and check for RIP timeouts
func (s *IPStack) RIPTimeoutCheck(timeout time.Duration) {
	// slog.Info("Starting RIP timeout check", "timeout", timeout)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		// Wait for ticker
		<-ticker.C
		// slog.Info("Checking RIP timeouts")

		// Check for timeouts
		for _, entry := range s.ForwardingTable.Entries {
			if entry.Source == SourceRIP {
				if time.Since(entry.LastUpdated) > timeout {
					// slog.Info("RIP route timed out", "prefix", entry.DestinationPrefix)
					s.ForwardingTable.RemoveRoute(entry.DestinationPrefix)
				}
			}
		}
	}
}