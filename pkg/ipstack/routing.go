package ipstack

import (
	// "log/slog"
	"net/netip"
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

func (s *IPStack) SendRIPRequest() {
	// Create a RIP request message
	// Send it to all RIP neighbors
}

func (s *IPStack) SendRIPResponse(dst netip.Addr, entries []RIPMessageEntry) {
	// Create a RIP response message
	// Implement split horizon with poisoned reverse
	// Send the message to the specified destination
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
	for _, entry := range ripMessage.entries {
		ipBytes := [4]byte{
			byte(entry.address >> 24),
			byte(entry.address >> 16),
			byte(entry.address >> 8),
			byte(entry.address),
		}
		destPrefix := netip.PrefixFrom(netip.AddrFrom4(ipBytes), int(entry.mask))
		cost := int(entry.cost) + 1 // Add 1 to the cost as per RIP rules

		if cost >= 16 {
			// Remove route if cost is infinity
			s.ForwardingTable.RemoveRoute(destPrefix)
		} else {
			// Add or update route
			s.ForwardingTable.AddRoute(ForwardingTableEntry{
				DestinationPrefix: destPrefix,
				NextHop:           sourceIP,
				Interface:         s.getInterfaceForIP(sourceIP),
				Metric:            cost,
				Source:            SourceRIP,
			})
		}
	}
}

func (s *IPStack) getInterfaceForIP(ip netip.Addr) string {
	for name, iface := range s.Interfaces {
		if iface.Netmask.Contains(ip) {
			return name
		}
	}
	return ""
}
