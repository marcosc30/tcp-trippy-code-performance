package tcpstack

import (
	"fmt"
	"net/netip"
    "github.com/smallnest/ringbuffer"
)

func (ts *TCPStack) HandlePacket(srcAddr, dstAddr netip.Addr, packet []byte) error {
	header, payload := ParseTCPHeader(packet)

	entry, err := ts.VFindTableEntry(dstAddr, header.DestPort, srcAddr, header.SourcePort)
	if err != nil {
		return err
	}

	switch entry.State {
	case TCP_LISTEN:
		fmt.Println("TCP_LISTEN")
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
		if header.Flags&TCP_ACK != 0 {
			handleEstablishedACK(ts, entry, header)
		}
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
        SID: ts.generateSID(),
		LocalAddress:  entry.LocalAddress,
		LocalPort:     entry.LocalPort,
		RemoteAddress: srcAddr,
		RemotePort:    header.SourcePort,
		SeqNum:        generateInitialSeqNum(),
		AckNum:        header.SeqNum + 1,
		tcpStack:      ts,
	}

	// Initialize send/receive state
	newSocket.snd = SND{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		ISS: newSocket.SeqNum,
		UNA: newSocket.SeqNum,
		NXT: newSocket.SeqNum + 1,  // +1 for SYN
		WND: BUFFER_SIZE,
	}
	
	newSocket.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		IRS: header.SeqNum,
		NXT: header.SeqNum + 1,  // +1 for SYN
		WND: header.WindowSize,
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

	// Send SYN-ACK with our window size
	synAckHeader := &TCPHeader{
		SourcePort: entry.LocalPort,
		DestPort:   header.SourcePort,
		SeqNum:     newSocket.SeqNum,
		AckNum:     newSocket.AckNum,
		DataOffset: 5,
		Flags:      TCP_SYN | TCP_ACK,
		WindowSize: newSocket.rcv.WND,  // Advertise our receive window
	}

	packet := serializeTCPPacket(synAckHeader, nil)
	ts.sendPacket(srcAddr, packet)

	// Add to accept queue when connection is established
	if header.Flags&TCP_ACK != 0 {
		listenSocket.acceptQueue <- newSocket
	}
}

func handleSYNACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)
	
	// Update socket state
	socket.AckNum = header.SeqNum + 1
	
	// Update receive state with peer's initial values
	socket.rcv.IRS = header.SeqNum
	socket.rcv.NXT = header.SeqNum + 1  // +1 for SYN
	socket.rcv.WND = header.WindowSize  // Store peer's advertised window
	
	// Update send state
	socket.snd.UNA = header.AckNum
	socket.snd.NXT = header.AckNum
	
	entry.State = TCP_ESTABLISHED

	// Send ACK
	ackHeader := &TCPHeader{
		SourcePort: entry.LocalPort,
		DestPort:   entry.RemotePort,
		SeqNum:     socket.snd.NXT,
		AckNum:     socket.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_ACK,
		WindowSize: socket.rcv.WND,  // Advertise our receive window
	}

	packet := serializeTCPPacket(ackHeader, nil)
	ts.sendPacket(entry.RemoteAddress, packet)
}

func handleData(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, payload []byte) {
	socket := entry.SocketStruct.(*NormalSocket)
	
	// Check if the sequence number is what we expect
	if header.SeqNum == socket.rcv.NXT {
		// Write the data to our receive buffer
		n, err := socket.rcv.buf.Write(payload)
		if err != nil {
			fmt.Printf("Error writing to receive buffer: %v\n", err)
			return
		}

		// Update our next expected sequence number
		socket.rcv.NXT += uint32(n)
		
		// Update receive window
		socket.rcv.WND = uint16(socket.rcv.buf.Free())

		// Send ACK
		ackHeader := &TCPHeader{
			SourcePort: entry.LocalPort,
			DestPort:   entry.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT,
			DataOffset: 5,
			Flags:      TCP_ACK,
			WindowSize: socket.rcv.WND,
		}

		packet := serializeTCPPacket(ackHeader, nil)
		ts.sendPacket(entry.RemoteAddress, packet)
	} else {
		// If we received out-of-order data, just send an ACK with the sequence 
		// number we're expecting. For now, we'll drop out-of-order segments
		ackHeader := &TCPHeader{
			SourcePort: entry.LocalPort,
			DestPort:   entry.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT, // What we're expecting
			DataOffset: 5,
			Flags:      TCP_ACK,
			WindowSize: socket.rcv.WND,
		}

		packet := serializeTCPPacket(ackHeader, nil)
		ts.sendPacket(entry.RemoteAddress, packet)
	}
}

func handleFIN(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	// TODO: Handle connection termination
}

func handleEstablishedACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)

    // Check if new ACK
	if header.AckNum > socket.snd.UNA {
		bytesAcked := header.AckNum - socket.snd.UNA
		
		socket.snd.UNA = header.AckNum
		
		buffer := make([]byte, bytesAcked)
		_, err := socket.snd.buf.Read(buffer)
		if err != nil {
			fmt.Printf("Error reading from send buffer: %v\n", err)
			return
		}
		
		socket.snd.WND = header.WindowSize
		
        // Send more data
		if socket.snd.buf.Length() > 0 {
			// TODO: Implement sending of buffered data
            // Will add when sending files and such
		}
	}
}

func handleACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)

	// Handle only the third packet of the three-way handshake
	entry.State = TCP_ESTABLISHED
	socket.SeqNum++
	socket.AckNum = header.SeqNum + 1

	// If this is from a listening socket's child, add to accept queue
	parentEntry, err := ts.VFindTableEntry(entry.LocalAddress, entry.LocalPort, netip.Addr{}, 0)
	if err == nil && parentEntry.State == TCP_LISTEN {
		listenSocket := parentEntry.SocketStruct.(*ListenSocket)
		listenSocket.acceptQueue <- socket
	}

	fmt.Printf("Connection established with %s:%d\n", entry.RemoteAddress, entry.RemotePort)
}