package ipstack

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"log/slog"
)

type IPPacket struct {
	SourceIP      netip.Addr
	DestinationIP netip.Addr
	TTL           uint8
	Protocol      Protocol
	Payload       []byte
	Checksum      uint16
}

type Protocol uint8

const (
	TEST_PROTOCOL Protocol = 1
	RIP_PROTOCOL  Protocol = 200
)


func CreatePacket(source string, destination string, ttl uint8, protocol Protocol, payload string) (IPPacket, error) {
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
		TTL:           ttl,
		Protocol:      protocol,
		Payload:       []byte(payload),
		Checksum:      0,
	}

	packet.Checksum = packet.CalculateChecksum()

	return packet, nil
}


func (p *IPPacket) Marshal() ([]byte, error) {
	// Marshal packet to byte array
	buf := new(bytes.Buffer)
	// Check the MarshalBinary for possible errors, 
	// I think it is the right function to use based on docs
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

	err = binary.Write(buf, binary.BigEndian, p.Checksum)
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

	sourceIP, ok := netip.AddrFromSlice(source_ip)
	if !ok {
		return IPPacket{}, err
	}
	packet.SourceIP = sourceIP

	// Read destination IP
	destination_ip := make([]byte, 4)
	err = binary.Read(buf, binary.BigEndian, &destination_ip)
	if err != nil {
		return IPPacket{}, errors.New("error unmarshalling destination IP")
	}

	packet.DestinationIP, ok = netip.AddrFromSlice(destination_ip)
	if !ok {
		return IPPacket{}, errors.New("error unmarshalling destination IP")
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

// This function validates a pakcet by checking TTL and checksum
func ValidatePacket(packet IPPacket) bool {
	if packet.TTL <= 0 {
		slog.Info("Invalid TTL")
		return false
	}

	checksum := packet.CalculateChecksum()
	if checksum != packet.Checksum {
		slog.Info("Invalid checksum")
		return false
	}
	return true
}

func (p *IPPacket) CalculateChecksum() uint16 {
	// Checksum is just based of header
	// TODO: Implement checksum
	return 0
}
