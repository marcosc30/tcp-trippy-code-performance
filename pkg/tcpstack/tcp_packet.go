package tcpstack

import (
	"encoding/binary"
	"net/netip"
	"time"
	"fmt"
)

type TCPHeader struct {
	SourcePort    uint16
	DestPort      uint16
	SeqNum        uint32
	AckNum        uint32
	DataOffset    uint8  // 4 bits
	Flags         uint8  // 8 bits
	WindowSize    uint16
	Checksum      uint16
	UrgentPtr     uint16
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
		SourcePort:    binary.BigEndian.Uint16(data[0:2]),
		DestPort:      binary.BigEndian.Uint16(data[2:4]),
		SeqNum:        binary.BigEndian.Uint32(data[4:8]),
		AckNum:        binary.BigEndian.Uint32(data[8:12]),
		DataOffset:    data[12] >> 4,
		Flags:         data[13],
		WindowSize:    binary.BigEndian.Uint16(data[14:16]),
		Checksum:      binary.BigEndian.Uint16(data[16:18]),
		UrgentPtr:     binary.BigEndian.Uint16(data[18:20]),
	}

	headerLen := int(header.DataOffset) * 4
	return header, data[headerLen:]
}

func (ts *TCPStack) HandlePacket(srcAddr, dstAddr netip.Addr, packet []byte) error {
    fmt.Printf("Received TCP packet from %s to %s\n", srcAddr, dstAddr)
	header, payload := ParseTCPHeader(packet)

	entry, err := ts.VFindTableEntry(dstAddr, header.DestPort, srcAddr, header.SourcePort)
	fmt.Printf("Found entry: %+v\n", entry)
	if err != nil {
		return err
	}

	switch entry.State {
	case TCP_LISTEN:
		if header.Flags&TCP_SYN != 0 {
			// Handle incoming connection
			handleSYN(ts, entry, header, srcAddr)
		}

	case TCP_SYN_SENT:
		if header.Flags&TCP_SYN != 0 && header.Flags&TCP_ACK != 0 {
			handleSYNACK(ts, entry, header)
		}

	case TCP_SYN_RECEIVED:
		if header.Flags&TCP_ACK != 0 {
			handleACK(ts, entry, header)
		}

	case TCP_ESTABLISHED:
		if len(payload) > 0 {
			handleData(ts, entry, header, payload)
		}
		if header.Flags&TCP_FIN != 0 {
			handleFIN(ts, entry, header)
		}
	}

	return nil
}

func handleSYN(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, srcAddr netip.Addr) {
	// Create new connection in SYN_RECEIVED state
	newSocket := &NormalSocket{
		LocalAddress:  entry.LocalAddress,
		LocalPort:     entry.LocalPort,
		RemoteAddress: srcAddr,
		RemotePort:    header.SourcePort,
		SeqNum:        generateInitialSeqNum(),
		AckNum:        header.SeqNum + 1,
		tcpStack:      ts,
	}

	// Get the listening socket to add to accept queue
	listenSocket := entry.SocketStruct.(*ListenSocket)

	newEntry := TCPTableEntry{
		LocalAddress:  entry.LocalAddress,
		LocalPort:     entry.LocalPort,
		RemoteAddress: srcAddr,
		RemotePort:    header.SourcePort,
		State:         TCP_SYN_RECEIVED,
		SocketStruct:  newSocket,
	}
	ts.VInsertTableEntry(newEntry)

	// Send SYN-ACK
	synAckHeader := &TCPHeader{
		SourcePort: entry.LocalPort,
		DestPort:   header.SourcePort,
		SeqNum:     newSocket.SeqNum,
		AckNum:     newSocket.AckNum,
		DataOffset: 5,
		Flags:      TCP_SYN | TCP_ACK,
		WindowSize: 65535,
	}

	packet := serializeTCPPacket(synAckHeader, nil)
	ts.sendPacket(srcAddr, packet)

	// Add to accept queue when connection is established
	if header.Flags&TCP_ACK != 0 {
		listenSocket.acceptQueue <- newSocket
	}
}

func handleSYNACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	entry.State = TCP_ESTABLISHED
	socket := entry.SocketStruct.(*NormalSocket)
	socket.AckNum = header.SeqNum + 1

	// Send ACK
	ackHeader := &TCPHeader{
		SourcePort: entry.LocalPort,
		DestPort:   entry.RemotePort,
		SeqNum:     socket.SeqNum + 1,
		AckNum:     socket.AckNum,
		DataOffset: 5,
		Flags:      TCP_ACK,
		WindowSize: 65535,
	}

	packet := serializeTCPPacket(ackHeader, nil)
	ts.sendPacket(entry.RemoteAddress, packet)
}

func handleData(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, payload []byte) {
	// TODO: Handle data reception and send ACK
}

func handleFIN(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	// TODO: Handle connection termination
}

func handleACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
    // Move to ESTABLISHED state
    entry.State = TCP_ESTABLISHED
    
    // Get the socket
    socket := entry.SocketStruct.(*NormalSocket)
    
    // Update sequence numbers
    socket.SeqNum++
    socket.AckNum = header.SeqNum + 1
    
    // If this is from a listening socket's child, add to accept queue
    // Find the parent listening socket
    parentEntry, err := ts.VFindTableEntry(entry.LocalAddress, entry.LocalPort, netip.Addr{}, 0)
    if err == nil && parentEntry.State == TCP_LISTEN {
        listenSocket := parentEntry.SocketStruct.(*ListenSocket)
        listenSocket.acceptQueue <- socket
    }
    
    fmt.Printf("Connection established with %s:%d\n", entry.RemoteAddress, entry.RemotePort)
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