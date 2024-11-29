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
	binary.BigEndian.PutUint16(packet[16:18], 0) // Zero checksum initially
	binary.BigEndian.PutUint16(packet[18:20], header.UrgentPtr)

	// Add payload if any
	if len(payload) > 0 {
		copy(packet[20:], payload)
	}

	return packet
}

func computeChecksum(srcIP, dstIP []byte, protocol uint8, tcpPacket []byte) uint16 {
	// Create pseudo header (12 bytes)
	pseudoHeader := make([]byte, 12)
	
	// Source IP
	copy(pseudoHeader[0:4], srcIP)
	// Destination IP
	copy(pseudoHeader[4:8], dstIP)
	// Zero byte
	pseudoHeader[8] = 0
	// Protocol
	pseudoHeader[9] = protocol
	// TCP length (header + data)
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(len(tcpPacket)))

	// Combine pseudo header and TCP segment for checksum calculation
	totalLength := len(pseudoHeader) + len(tcpPacket)
	if totalLength%2 != 0 {
		totalLength++
	}
	
	checksumData := make([]byte, totalLength)
	copy(checksumData[0:], pseudoHeader)
	copy(checksumData[12:], tcpPacket)

	// Calculate checksum
	var sum uint32
	for i := 0; i < len(checksumData)-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(checksumData[i:i+2]))
	}

	// Add carried over bits
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	// One's complement
	return ^uint16(sum) 
}

func generateInitialSeqNum() uint32 {
	// For now, use a simple random number
	return uint32(time.Now().UnixNano() & 0xFFFFFFFF)
}

