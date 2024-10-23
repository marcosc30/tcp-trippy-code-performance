package ipstack

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
)

// Here, we also define the interface struct
type Interface struct {
	Name      string
	IPAddr    netip.Addr
	Netmask   netip.Prefix
	UDPAddr   *net.UDPAddr
	Socket    *net.UDPConn
	Neighbors map[netip.Addr]*net.UDPAddr // Neighbor IP to UDP address mapping
	Down      bool
}

func (i *Interface) SendPacket(packet *IPPacket, nextHop netip.Addr) error {
	if i.Down {
		return errors.New("interface is down")
	}

	// Send packet to nextHop
	marshalled_packet, err := packet.Marshal()
	if err != nil {
		return err
	}

	// Check if nextHop is in table
	if _, ok := i.Neighbors[nextHop]; !ok {
		return errors.New("nextHop not in neighbors table")
	}

	_, err = i.Socket.WriteToUDP(marshalled_packet, i.Neighbors[nextHop])
	if err != nil {
		return err
	}

	return nil
}

// Intended for interfaces to use to listen for incoming packets
func InterfaceListen(i *Interface, stack *IPStack) {
	// The packet handler function will likely be just one that holds on to it if it is the destination or forwards it if not
	// Listen on interface for packets
	for {
		if i.Down {
			continue
		}

		slog.Debug("Listening for packets", "Interface", i.Name)

		buffer := make([]byte, 1024)
		n, _, err := i.Socket.ReadFromUDP(buffer)
		if err != nil {
			// Handle error
			slog.Error("Error reading from interface", "error", err, "interface", i.Name)
		}

		slog.Debug("Received packet", "Interface", i.Name, "Packet", buffer[:n])

		packet, err := UnmarshalPacket(buffer[:n])
		if err != nil {
			slog.Error("Error unmarshalling packet", "Interface", i.Name, "error", err)
		}

		ReceivePacket(&packet, stack)
	}
}
