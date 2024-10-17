package ipstack

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"time"
)

// Table for RIP
type RIPTableEntry struct {
	destAddr netip.Prefix
	nextHop  netip.Addr
	cost     int
	timer    time.Time
	valid    bool // This is to quickly check if a route is valid (so if they haven't updated yet, or )
}

type RIPTable struct {
	Entries            []RIPTableEntry
	UpdateNeighbors    []netip.Addr
	PeriodicUpdateRate time.Duration
	TimeoutThreshold   time.Duration
	Mutex              *sync.Mutex
}

// RIP Functions

// General packet handler
func HandleRIPPacket(packet *IPPacket, stack *IPStack) {
	ripMessage, err := UnmarshalRIPMessage(packet.Payload)
	if err != nil {
		slog.Error("Error unmarshalling RIP message", "error", err)
		return
	}

	slog.Info("Received RIP message", "message", ripMessage)

	switch ripMessage.command {
	case 1: // Request
		stack.SendRIPResponse(packet, ripMessage)
	case 2: // Response
		stack.ProcessRIPResponse(packet, ripMessage)
	}
}

func (stack *IPStack) SendRIPResponse(packet *IPPacket, ripMessage RIPMessage) {
	return
}

func (stack *IPStack) ProcessRIPResponse(packet *IPPacket, ripMessage RIPMessage) {
	return
}

// Finds the next hop for a destination address
func (ripTable *RIPTable) Lookup(destAddr netip.Addr) (netip.Addr, error) {
	ripTable.Mutex.Lock()
	defer ripTable.Mutex.Unlock()

	// For all prefix matches, find the one with the lowest cost
	lowestCost := 16 // Max is 16 (= infinity)
	lowestHop := netip.Addr{}

	for _, entry := range ripTable.Entries {
		if entry.destAddr.Contains(destAddr) {
			if entry.cost < lowestCost {
				lowestCost = entry.cost
				lowestHop = entry.nextHop
			}
		}
	}

	if lowestCost == 16 {
		return netip.Addr{}, errors.New("no route to destination")
	}

	return lowestHop, nil
}
