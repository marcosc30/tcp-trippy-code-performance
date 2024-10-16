package ipstack

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
)

// This file focuses on sending the packets themselves, given adequete port information

func CreatePacket(source string, destination string, protocol uint8, payload string) (IPPacket, error) {
	// Create packet struct

	source_ip, err := netip.ParseAddr(source)
	if err != nil {
		return IPPacket{}, err
	}

	destination_ip, err := netip.ParseAddr(destination)
	if err != nil {
		return IPPacket{}, err
	}

	packet := IPPacket{
		SourceIP:      netip.Addr(source_ip),
		DestinationIP: netip.Addr(destination_ip),
		TTL:           64,
		Protocol:      protocol,
		Payload:       []byte(payload),
		Checksum:      0,
	}

	packet.Checksum = packet.CalculateChecksum()

	return packet, nil
}

// We can define the packet struct here and marshal it to a byte array to send it

type IPPacket struct {
	SourceIP      netip.Addr
	DestinationIP netip.Addr
	TTL           uint8
	Protocol      uint8 // This would be UDP or TCP later on
	Payload       []byte
	Checksum      uint16
}

func (p *IPPacket) Marshal() ([]byte, error) {
	// Marshal packet to byte array
	buf := new(bytes.Buffer)
	// Check the MarshalBinary for possible errors, I think it is the right function to use based on doc
	bytes1, err := p.SourceIP.MarshalBinary()
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, bytes1)
	if err != nil {
		return nil, err
	}

	bytes2, err := p.DestinationIP.MarshalBinary()
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, bytes2)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, p.TTL)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, p.Protocol)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, p.Payload)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil

}

func UnmarshalPacket(data []byte) (IPPacket, error) {
	// Unmarshal byte array to packet
	packet := IPPacket{}

	buf := bytes.NewBuffer(data)

	// Read source IP
	source_ip := make([]byte, 4)
	err := binary.Read(buf, binary.BigEndian, &source_ip)
	if err != nil {
		return IPPacket{}, err
	}

	packet.SourceIP, err = netip.ParseAddr(string(source_ip))
	if err != nil {
		return IPPacket{}, err
	}

	// Read destination IP
	destination_ip := make([]byte, 4)
	err = binary.Read(buf, binary.BigEndian, &destination_ip)
	if err != nil {
		return IPPacket{}, err
	}
	packet.DestinationIP, err = netip.ParseAddr(string(destination_ip))
	if err != nil {
		return IPPacket{}, err
	}

	// Read TTL
	err = binary.Read(buf, binary.BigEndian, &packet.TTL)
	if err != nil {
		return IPPacket{}, err
	}

	// Read protocol
	err = binary.Read(buf, binary.BigEndian, &packet.Protocol)
	if err != nil {
		return IPPacket{}, err
	}

	// Read payload
	packet.Payload = buf.Bytes()

	return packet, nil
}

func (p *IPPacket) CalculateChecksum() uint16 {
	// Checksum is just based of header
	// TODO: Implement checksum
	return 0
}


// Here, we also define the interface struct
type Interface struct {
	Name      string
	IPAddress netip.Addr
	Netmask   netip.Prefix
	UDPAddr   *netip.AddrPort
	Socket    *net.UDPConn
	Neighbors map[netip.Addr]*netip.AddrPort // Neighbor IP to UDP address mapping
	Down      bool
}

// We define a method on the interface to send to a neighbor
func (i *Interface) SendToNeighbor(packet *IPPacket, neighbor netip.Addr) error {
	// Send packet to neighbor
	marshalled_packet, err := packet.Marshal()
	if err != nil {
		return err
	}
	_, err = i.Socket.WriteToUDP(marshalled_packet, i.Neighbors[neighbor])
	if err != nil {
		return err
	}

	return nil
}

// Intended for interfaces to use to listen for incoming packets
func InterfaceListen(i *Interface, packetHandler func(*IPPacket)) {
	// The packet handler function will likely be just one that holds on to it if it is the destination or forwards it if not
	// Listen on interface for packets
	for {
		buffer := make([]byte, 1024)
		n, _, err := i.Socket.ReadFromUDP(buffer)
		if err != nil {
			// Handle error
		}

		packet := IPPacket{
			SourceIP:      i.IPAddress,
			DestinationIP: netip.Addr{},
			TTL:           64,
			Protocol:      0,
			Payload:       buffer[:n],
		}

		packetHandler(&packet)
	}
}
