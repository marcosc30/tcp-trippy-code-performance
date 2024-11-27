package tcpstack

import (
	"encoding/binary"
	"fmt"
	"time"
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
	if len(payload) > MAX_TCP_PAYLOAD {
		fmt.Printf("Payload is too large, max is %d, got %d\n", MAX_TCP_PAYLOAD, len(payload))
		return nil
	}
	
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
	binary.BigEndian.PutUint16(packet[16:18], 0) // Checksum 
	binary.BigEndian.PutUint16(packet[18:20], header.UrgentPtr)

	// Add payload if any
	if len(payload) > 0 {
		copy(packet[20:], payload)
	}

	return packet
}

func computeChecksum(packet []byte) uint16 {
	// TODO: Implement checksum computation
	// Might have to change serializeTCPPacket to include IP pseudo header
	return 0
}

func generateInitialSeqNum() uint32 {
	// For now, use a simple random number
	return uint32(time.Now().UnixNano() & 0xFFFFFFFF)
}

