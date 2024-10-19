package ipstack

import (
	"errors"
	"log"
	"net/netip"

	ipv4header "github.com/brown-csci1680/iptcp-headers"
	"github.com/google/netstack/tcpip/header"
)

type IPPacket struct {
	SourceIP      netip.Addr
	DestinationIP netip.Addr
	TTL           uint8
	Protocol      Protocol
	Payload       []byte
	Checksum      int
}

type Protocol uint8

const (
	TEST_PROTOCOL Protocol = 1
	RIP_PROTOCOL  Protocol = 200
)

// Creates a new packet struct with the given source, destination, ttl, protocol, and payload
func CreatePacket(source_ip netip.Addr, destination_ip netip.Addr, ttl uint8, protocol Protocol, payload string) (IPPacket, error) {
	packet := IPPacket{
		SourceIP:      source_ip,
		DestinationIP: destination_ip,
		TTL:           ttl,
		Protocol:      protocol,
		Payload:       []byte(payload),
	}

	packet.Checksum = packet.CalculateChecksum()

	return packet, nil
}

// Marshals the packet into a byte array that corresponds to an actual IP packet
func (p *IPPacket) Marshal() ([]byte, error) {
	hdr := ipv4header.IPv4Header{
		Version:  4,
		Len:      20, // Header length is always 20 when no IP options
		TOS:      0,
		TotalLen: ipv4header.HeaderLen + len(p.Payload),
		ID:       0,
		Flags:    0,
		FragOff:  0,
		TTL:      int(p.TTL),
		Protocol: int(p.Protocol),
		Src:      p.SourceIP,
		Dst:      p.DestinationIP,
		Options:  []byte{},
		Checksum: p.Checksum,
	}

	// Assemble the header into a byte array
	headerBytes, err := hdr.Marshal()
	if err != nil {
		log.Fatalln("Error marshalling header:  ", err)
	}

	// Append header + message into one byte array
	bytesToSend := make([]byte, 0, len(headerBytes)+len(p.Payload))
	bytesToSend = append(bytesToSend, headerBytes...)
	bytesToSend = append(bytesToSend, []byte(p.Payload)...)

	return bytesToSend, nil
}

// Unmarshals a byte array into an IPPacket struct
func UnmarshalPacket(data []byte) (IPPacket, error) {
	hdr, err := ipv4header.ParseHeader(data)
	if err != nil {
		return IPPacket{}, err
	}

	payload := data[hdr.Len:]
	packet := IPPacket{
		SourceIP:      hdr.Src,
		DestinationIP: hdr.Dst,
		TTL:           uint8(hdr.TTL),
		Protocol:      Protocol(hdr.Protocol),
		Payload:       payload,
		Checksum:      hdr.Checksum,
	}

	if !ValidatePacket(packet) {
		return IPPacket{}, errors.New("invalid packet")
	}

	return packet, nil
}

// This function validates a pakcet by checking TTL and checksum
func ValidatePacket(packet IPPacket) bool {
	log.Println(packet.TTL, packet.Protocol)
	if packet.TTL == 0 {
		log.Println("TTL is 0")
		return false
	}

	if packet.CalculateChecksum() != packet.Checksum {
		log.Println("Checksum is invalid")
		return false
	}

	return true
}

// This function calculates the checksum of the packet
func (p *IPPacket) CalculateChecksum() int {
	hdr := ipv4header.IPv4Header{
		Version:  4,
		Len:      20, // Header length is always 20 when no IP options
		TOS:      0,
		TotalLen: ipv4header.HeaderLen + len(p.Payload),
		ID:       0,
		Flags:    0,
		FragOff:  0,
		TTL:      int(p.TTL),
		Protocol: int(p.Protocol),
		Checksum: 0, // Should be 0 until checksum is computed
		Src:      p.SourceIP,
		Dst:      p.DestinationIP,
		Options:  []byte{},
	}

	bytes_before_checksum, err := hdr.Marshal()
	if err != nil {
		return 0
	}

	checksum := header.Checksum(bytes_before_checksum, 0)
	return int(checksum)
}
