package ipstack

import (
	"ip-rip-in-peace/pkg/lnxconfig"
	"net"
	"net/netip"
	"sync"
	"time"
)

// InitNode parses a host's lnx file and returns an error for unsuccessful, maybe will add a return for the port for sending RIP updates if it is a router

// func CreateIPConfig() lnxconfig.IPConfig {
// 	// Parse lnx file
// 	// Return IPConfig
// 	// May not be necessary given their parser, depends on how extensive it is (I think it makes the config file for you so may not be neccessary)

// }

func InitNode(fileName string) (*IPStack, error) {
	// Parse lnx file
	ipconfig, err := lnxconfig.ParseConfig(fileName)
	if err != nil {
		return nil, err
	}

	// Create IPStack
	ipstack := IPStack{
		Mutex: sync.RWMutex{},
		IPConfig: ipconfig,
	}

	ipstack.ForwardingTable = &ForwardingTable{
		Entries: make([]ForwardingTableEntry, 0),
	}

	// Create interfaces
	ip_interfaces := make(map[string]*Interface)

	for _, iface := range ipconfig.Interfaces {
		inter := Interface{
			Name:      iface.Name,
			IPAddress: iface.AssignedIP,
			Netmask:   iface.AssignedPrefix,
			UDPAddr:   &iface.UDPAddr,
			Socket:    nil,
			Neighbors: make(map[netip.Addr]*net.UDPAddr),
		}

		// Create UDP socket
		udpaddr := net.UDPAddrFromAddrPort(*inter.UDPAddr)
		conn, err := net.ListenUDP("udp", udpaddr)
		if err != nil {
			return nil, err
		}
		inter.Socket = conn

		ip_interfaces[iface.Name] = &inter // iface.Name might not be unique, so check that

		// We also add the interfaces to the forwarding table
		entry := ForwardingTableEntry{
			DestinationPrefix: iface.AssignedPrefix,
			NextHop:           iface.AssignedIP,
			Interface:         iface.Name,
		}

		ipstack.ForwardingTable.Entries = append(ipstack.ForwardingTable.Entries, entry)
	}
	ipstack.Interfaces = ip_interfaces

	// Assign IP addresses

	// Populate forwarding table

	for _, neighbor := range ipconfig.Neighbors {
		prefix_length := 32
		entry := ForwardingTableEntry{
			DestinationPrefix: netip.PrefixFrom(neighbor.DestAddr, prefix_length),
			NextHop:           neighbor.DestAddr,
			Interface:         neighbor.InterfaceName,
		}
		ipstack.ForwardingTable.Entries = append(ipstack.ForwardingTable.Entries, entry)
	}

	return &ipstack, nil
}

func Init_RIP(ipconfig lnxconfig.IPConfig) (*RIPTable, error) {
	// Create routing table
	// A different function in routing.go is used to send RIP updates, with routers needing to have it running on a separate thread
	ripTable := RIPTable{
		Entries: make([]RIPTableEntry, 0),
	}

	// Add entries to routing table, 
	// The valid part of the entries may be unnecessary, as is adding neighbors to the entries list (we can just add new neighbors
	// and remove invalid ones as RIP updates come in)
	for _, neighbor := range ipconfig.Neighbors {
		entry := RIPTableEntry{
			destAddr: netip.PrefixFrom(neighbor.DestAddr, 32),
			nextHop:  neighbor.DestAddr,
			cost:     1,
			timer:    time.Now(),
			valid:   false,
		}
		ripTable.Entries = append(ripTable.Entries, entry)
	}

	ripTable.RipPeriodicUpdateRate = ipconfig.RipPeriodicUpdateRate
	ripTable.RipTimeoutThreshold = ipconfig.RipTimeoutThreshold
	ripTable.UpdateNeighbors = ipconfig.RipNeighbors

	return &ripTable, nil
}