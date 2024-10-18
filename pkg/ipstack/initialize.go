package ipstack

import (
	"ip-rip-in-peace/pkg/lnxconfig"
	"net"
	"net/netip"
	"sync"
	// "log/slog"
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

	// Parse IP Config
	ipstack := IPStack{}

	ipstack.IPConfig = ipconfig

	// Create handlers
	ipstack.Handlers = make(map[Protocol]HandlerFunc)

	ipstack.ForwardingTable = &ForwardingTable{
		Entries: make([]ForwardingTableEntry, 0),
		Mutex:   sync.RWMutex{},
	}

	// Create interfaces
	ip_interfaces := make(map[string]*Interface)

	for _, iface := range ipconfig.Interfaces {
		// Convert from netip.Addr to net.UDPAddr
		udpaddr := net.UDPAddr{
			IP:   iface.UDPAddr.Addr().AsSlice(),
			Port: int(iface.UDPAddr.Port()),
		}

		inter := Interface{
			Name:      iface.Name,
			IPAddr:    iface.AssignedIP,
			Netmask:   iface.AssignedPrefix,
			UDPAddr:   &udpaddr,
			Socket:    nil,
			Neighbors: make(map[netip.Addr]*net.UDPAddr),
		}

		// Create UDP socket
		conn, err := net.ListenUDP("udp", &udpaddr)
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

	// Add the neighbors to the interfaces
	for _, neighbor := range ipconfig.Neighbors {
		neighborUDP := net.UDPAddr{
			IP:   neighbor.UDPAddr.Addr().AsSlice(),
			Port: int(neighbor.UDPAddr.Port()),
		}
		ipstack.Interfaces[neighbor.InterfaceName].Neighbors[neighbor.DestAddr] = &neighborUDP
	}

	// Assign IP addresses

	// // Populate forwarding table
	// for _, neighbor := range ipconfig.Neighbors {
	// 	prefix_length := 32
	// 	entry := ForwardingTableEntry{
	// 		DestinationPrefix: netip.PrefixFrom(neighbor.DestAddr, prefix_length),
	// 		NextHop:           neighbor.DestAddr,
	// 		Interface:         neighbor.InterfaceName,
	// 	}
	// 	ipstack.ForwardingTable.Entries = append(ipstack.ForwardingTable.Entries, entry)
	// }
	
	// Add connected routes
	for _, iface := range ipconfig.Interfaces {
		ipstack.ForwardingTable.AddRoute(ForwardingTableEntry{
			DestinationPrefix: iface.AssignedPrefix,
			NextHop:           iface.AssignedIP,
			Interface:         iface.Name,
			Metric:            0,
			Source:            SourceConnected,
		})
	}

	// Add static routes
	for prefix, nextHop := range ipconfig.StaticRoutes {
		ipstack.ForwardingTable.AddRoute(ForwardingTableEntry{
			DestinationPrefix: prefix,
			NextHop:           nextHop,
			Interface:         ipstack.getInterfaceForIP(nextHop),
			Metric:            1,
			Source:            SourceStatic,
		})
	}

	return &ipstack, nil
}

// func (ipstack *IPStack) InitializeHostDefault() {
// 	// We grab the neighbor, which we assume is only the router since it is a host
// 	router_neighbor := ipstack.ForwardingTable.Entries[1]

// 	// We copy the neighbor's entry in the forwarding table but make its prefix 0 to be default
// 	default_entry := ForwardingTableEntry{
// 		DestinationPrefix: netip.PrefixFrom(router_neighbor.DestinationPrefix.Addr(), 0),
// 		NextHop:           router_neighbor.NextHop,
// 		Interface:         router_neighbor.Interface,
// 	}

// 	ipstack.ForwardingTable.Entries = append(ipstack.ForwardingTable.Entries, default_entry)
// }

// func InitRIP(ipconfig lnxconfig.IPConfig) (*RIPTable, error) {
// 	// Create routing table
// 	// A different function in routing.go is used to send RIP updates, with routers needing to have it running on a separate thread
// 	ripTable := RIPTable{
// 		Entries: make([]RIPTableEntry, 0),
// 		Mutex:   &sync.Mutex{},
// 	}

// 	// TODO: Do we need this?
// 	// Add entries to routing table,
// 	// The valid part of the entries may be unnecessary, as is adding neighbors to the entries list (we can just add new neighbors
// 	// and remove invalid ones as RIP updates come in)
// 	for _, neighbor := range ipconfig.Neighbors {
// 		entry := RIPTableEntry{
// 			destAddr: netip.PrefixFrom(neighbor.DestAddr, 32),
// 			nextHop:  neighbor.DestAddr,
// 			cost:     1,
// 			timer:    time.Now(),
// 			valid:    false,
// 		}
// 		ripTable.Entries = append(ripTable.Entries, entry)
// 	}

// 	ripTable.PeriodicUpdateRate = ipconfig.RipPeriodicUpdateRate
// 	ripTable.TimeoutThreshold = ipconfig.RipTimeoutThreshold
// 	ripTable.UpdateNeighbors = ipconfig.RipNeighbors

// 	return &ripTable, nil
// }
