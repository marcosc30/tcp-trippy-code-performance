package ipstack

import (
	"log/slog"
	"net/netip"
	"time"
)

// Table for RIP

// func NewRIPTable(stack *IPStack) *RIPTable {
// 	return &RIPTable{
// 		Entries:            make(map[netip.Prefix]*RIPTableEntry),
// 		UpdateNeighbors:    make([]netip.Addr, 0),
// 		PeriodicUpdateRate: 5 * time.Second,
// 		TimeoutThreshold:   12 * time.Second,
// 		Mutex:              &sync.RWMutex{},
// 		stack:              stack,
// 	}
// }

// func (rt *RIPTable) AddOrUpdateEntry(prefix netip.Prefix, nextHop netip.Addr, cost int) {
// 	rt.Mutex.Lock()
// 	defer rt.Mutex.Unlock()

// 	entry, exists := rt.Entries[prefix]
// 	if !exists || cost < entry.cost {
// 		if exists {
// 			entry.timer.Stop()
// 		}
// 		timer := time.AfterFunc(rt.TimeoutThreshold, func() {
// 			rt.ExpireRoute(prefix)
// 		})
// 		rt.Entries[prefix] = &RIPTableEntry{
// 			destAddr: prefix,
// 			nextHop:  nextHop,
// 			cost:     cost,
// 			timer:    timer,
// 			valid:    true,
// 		}
// 		rt.TriggerUpdate()
// 	} else if exists {
// 		entry.timer.Reset(rt.TimeoutThreshold)
// 	}
// }

// func (rt *RIPTable) ExpireRoute(prefix netip.Prefix) {
// 	rt.Mutex.Lock()
// 	defer rt.Mutex.Unlock()

// 	entry, exists := rt.Entries[prefix]
// 	if exists {
// 		entry.cost = 16 // Set cost to infinity
// 		entry.valid = false
// 		rt.TriggerUpdate()
// 		delete(rt.Entries, prefix)
// 	}
// }

// func (rt *RIPTable) TriggerUpdate() {
// 	// Implement triggered update logic here
// 	// This should send updates to all neighbors with only the changed routes
// }

// func (rt *RIPTable) StartPeriodicUpdates() {
// 	ticker := time.NewTicker(rt.PeriodicUpdateRate)
// 	go func() {
// 		for range ticker.C {
// 			rt.SendPeriodicUpdate()
// 		}
// 	}()
// }

// func (rt *RIPTable) SendPeriodicUpdate() {
// 	// Implement periodic update logic here
// 	// This should send the entire routing table to all neighbors
// }

// // Finds the next hop for a destination address
// func (ripTable *RIPTable) Lookup(destAddr netip.Addr) (netip.Addr, error) {
// 	ripTable.Mutex.Lock()
// 	defer ripTable.Mutex.Unlock()

// 	// For all prefix matches, find the one with the lowest cost
// 	lowestCost := 16 // Max is 16 (= infinity)
// 	lowestHop := netip.Addr{}

// 	for _, entry := range ripTable.Entries {
// 		if entry.destAddr.Contains(destAddr) {
// 			if entry.cost < lowestCost {
// 				lowestCost = entry.cost
// 				lowestHop = entry.nextHop
// 			}
// 		}
// 	}

// 	if lowestCost == 16 {
// 		return netip.Addr{}, errors.New("no route to destination")
// 	}

// 	return lowestHop, nil
// }


// Goroutine to send periodic RIP updates
func (s *IPStack) PeriodicUpdate(updateRate time.Duration) {
	slog.Info("Starting periodic update", "updateRate", updateRate)
	ticker := time.NewTicker(updateRate)
	defer ticker.Stop()
	
	for {
		// Wait for ticker
		<-ticker.C
		slog.Info("Sending periodic RIP update")

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
	slog.Info("Received RIP packet", "packet", packet)
	ripMessage, err := UnmarshalRIPMessage(packet.Payload)
	if err != nil {
		slog.Error("Error unmarshalling RIP message", "error", err)
		return
	}

	switch ripMessage.command {
	case RIP_REQUEST:
		stack.SendRIPResponse(packet.SourceIP, stack.GetAllRIPEntries())
	case RIP_RESPONSE:
		stack.ProcessRIPResponse(packet.SourceIP, ripMessage)
	}
}

// Send RIP Request to all neighbors
func (s *IPStack) SendRIPRequest() {
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
		packet := IPPacket{
			SourceIP: iface.IPAddr,
			DestinationIP: netip.Addr{},
			TTL: 1,
			Protocol: RIP_PROTOCOL,
			Payload: marshalled_message,
		}
		iface.SendPacket(&packet, netip.Addr{})
	}
}


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

	err = s.SendIP(dst, RIP_PROTOCOL, 1, marshalled_message)
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

	for _, entry := range ripMessage.entries {
		destAddr := uint32ToNetipAddr(entry.address)
		destPrefix := netip.PrefixFrom(destAddr, int(entry.mask))
		cost := int(entry.cost) + 1

		if cost >= 16 {
			// TODO: Check this, should we remove or just not add

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
			// Add or update route
			oldEntry, exists := s.ForwardingTable.Lookup(destPrefix)
			if !exists || cost < oldEntry.Metric {
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