package tcpstack

import (
	"encoding/binary"
	"time"
	"unsafe"
)

const MAX_TCP_PAYLOAD = 1400 - int(unsafe.Sizeof(TCPHeader{})) - 20 // 20 is size of IP header

type TCPHeader struct {
	SourcePort uint16
	DestPort   uint16
	SeqNum     uint32
	AckNum     uint32
	DataOffset uint8 // 4 bits
	Flags      uint8 // 8 bits
	WindowSize uint16
	Checksum   uint16
	UrgentPtr  uint16
}

const (
	TCP_FIN = 1 << 0
	TCP_SYN = 1 << 1
	TCP_RST = 1 << 2
	TCP_PSH = 1 << 3
	TCP_ACK = 1 << 4
)

func ParseTCPHeader(data []byte) (*TCPHeader, []byte) {
	header := &TCPHeader{
		SourcePort: binary.BigEndian.Uint16(data[0:2]),
		DestPort:   binary.BigEndian.Uint16(data[2:4]),
		SeqNum:     binary.BigEndian.Uint32(data[4:8]),
		AckNum:     binary.BigEndian.Uint32(data[8:12]),
		DataOffset: data[12] >> 4,
		Flags:      data[13],
		WindowSize: binary.BigEndian.Uint16(data[14:16]),
		Checksum:   binary.BigEndian.Uint16(data[16:18]),
		UrgentPtr:  binary.BigEndian.Uint16(data[18:20]),
	}

	headerLen := int(header.DataOffset) * 4
	return header, data[headerLen:]
}

// Add these helper functions
func serializeTCPPacket(header *TCPHeader, payload []byte) []byte {
	// 20 bytes for header
	packet := make([]byte, 20+len(payload))

	// Write header fields
	binary.BigEndian.PutUint16(packet[0:2], header.SourcePort)
	binary.BigEndian.PutUint16(packet[2:4], header.DestPort)
	binary.BigEndian.PutUint32(packet[4:8], header.SeqNum)
	binary.BigEndian.PutUint32(packet[8:12], header.AckNum)

	// Data offset and flags
	packet[12] = header.DataOffset << 4
	packet[13] = header.Flags

	binary.BigEndian.PutUint16(packet[14:16], header.WindowSize)
	binary.BigEndian.PutUint16(packet[16:18], 0) // Checksum (computed later)
	binary.BigEndian.PutUint16(packet[18:20], header.UrgentPtr)

	// Add payload if any
	if len(payload) > 0 {
		copy(packet[20:], payload)
	}

	return packet
}

func generateInitialSeqNum() uint32 {
	// For now, use a simple random number
	return uint32(time.Now().UnixNano() & 0xFFFFFFFF)
}

func generateEphemeralPort() uint16 {
	// Start with a fixed port for testing
	return 49152
}
