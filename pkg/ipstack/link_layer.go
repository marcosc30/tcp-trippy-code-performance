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
	}

	return packet, nil
}

// We can define the packet struct here and marshal it to a byte array to send it

type IPPacket struct {
	SourceIP      netip.Addr
	DestinationIP netip.Addr
	TTL           uint8
	Protocol      uint8 // This would be UDP or TCP later on
	Payload       []byte
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

// Here, we also define the interface struct
type Interface struct {
	Name      string
	IPAddress netip.Addr
	Netmask   netip.Prefix
	UDPAddr   *net.UDPAddr
	Socket    *net.UDPConn
	Neighbors map[netip.Addr]*net.UDPAddr // Neighbor IP to UDP address mapping
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