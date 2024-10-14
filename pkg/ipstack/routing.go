package ipstack

import (
	"net/netip"
	"time"
)

// Here, we will define all functions and structs related specifically to routers, mainly RIP and the structs needed for routers


type RIPTableEntry struct {
	destAddr netip.Prefix
	nextHop  netip.Addr
	cost     int
	timer    time.Time
	valid   bool // This is to quickly check if a route is valid (so if they haven't updated yet, or )
}

type RIPTable struct {
	Entries []RIPTableEntry
	UpdateNeighbors []netip.Addr
	RipPeriodicUpdateRate time.Duration
	RipTimeoutThreshold   time.Duration
}