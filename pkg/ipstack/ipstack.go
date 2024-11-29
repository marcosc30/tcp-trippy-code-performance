package ipstack

import (
	"errors"
	"ip-rip-in-peace/pkg/lnxconfig"
	"net/netip"
	"sync"
	// "log/slog"
)

type IPStack struct {
	Interfaces      map[string]*Interface
	ForwardingTable *ForwardingTable
	// Maybe a handler function as well for routers sending RIP updates?
	Mutex    sync.RWMutex        // Protects shared resources
	IPConfig *lnxconfig.IPConfig // We add this in case we need to access some information like TCP or router timing parameters
	Handlers map[Protocol]HandlerFunc
}

type HandlerFunc func(*IPPacket, *IPStack)

func (s *IPStack) SendIP(dst netip.Addr, protocol Protocol, ttl uint8, data []byte) error {
	// We treat it the same
	interfaceName, _ := s.ForwardingTable.NextHop(dst)
	if interfaceName == "" {
		return errors.New("no route to destination")
	}

	nextIF := s.Interfaces[interfaceName]

	// We increment TTL by one to counter the decrement in ReceivePacket
	packet, err := CreatePacket(nextIF.IPAddr, dst, ttl, protocol, data)
	if err != nil {
		return err
	}

	ReceivePacket(&packet, s)

	return nil
}

func (s *IPStack) RegisterHandler(protocol Protocol, handler HandlerFunc) {
	s.Handlers[protocol] = handler
}

func (s *IPStack) HandlePacket(packet *IPPacket) {
	// Check if we have a handler for this protocol
	handler, ok := s.Handlers[packet.Protocol]
	if !ok {
		// Drop packet
		return
	}

	handler(packet, s)
}

func ReceivePacket(packet *IPPacket, ipstack *IPStack) {
	// fmt.Println("Received packet")
	//fmt.Println("Received packet from: ", packet.SourceIP, "to: ", packet.DestinationIP, "protocol: ", packet.Protocol)
	// slog.Info("Received packet", "source", packet.SourceIP, "destination", packet.DestinationIP, "protocol", packet.Protocol, "ttl", packet.TTL)
	// 1. Validate packet
	if !ValidatePacket(*packet) {
		return
	}

	// 2. For me? Check all interfaces
	for _, iface := range ipstack.Interfaces {
		if iface.Down {
			continue
		}

		if iface.IPAddr == packet.DestinationIP {
			// slog.Info("Packet is for me")
			ipstack.HandlePacket(packet)
			return
		}
	}

	// slog.Info("Packet not for me")

	// 3. Check if the destination is on a directly connected network
	// if packet.Protocol != RIP_PROTOCOL {
	// slog.Info("Forwarding packet")
	// }
	for _, iface := range ipstack.Interfaces {
		if iface.Netmask.Contains(packet.DestinationIP) {
			//fmt.Println("Destination is on this network")
			// Destination is on this network, send directly
			nextIF := iface
			packet.TTL--
			packet.Checksum = packet.CalculateChecksum()
			nextIF.SendPacket(packet, packet.DestinationIP)
			return
		}
	}

	//fmt.Println("Forwarding packet")

	// 4. Forward packet
	interfaceName, nextHop := ipstack.ForwardingTable.NextHop(packet.DestinationIP)

	if interfaceName == "" {
		// Drop packet if no route found
		return
	}

	nextIF := ipstack.Interfaces[interfaceName]

	packet.TTL--
	packet.Checksum = packet.CalculateChecksum()

	nextIF.SendPacket(packet, nextHop)
}
