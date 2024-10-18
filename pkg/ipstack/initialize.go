package ipstack

import (
	"ip-rip-in-peace/pkg/lnxconfig"
	"net"
	"net/netip"
	"sync"
	// "log/slog"
)

// InitNode parses a host's lnx file and returns an error for unsuccessful, maybe will add a return for the port for sending RIP updates if it is a router
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