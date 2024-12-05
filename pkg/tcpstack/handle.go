package tcpstack

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/smallnest/ringbuffer"
)

func (ts *TCPStack) HandlePacket(srcAddr, dstAddr netip.Addr, packet []byte) error {
	header, payload := ParseTCPHeader(packet)

	entry, err := ts.VFindTableEntry(dstAddr, header.DestPort, srcAddr, header.SourcePort)
	if err != nil {
		return err
	}

	//fmt.Println("Handling packet for", entry.LocalAddress, entry.LocalPort, entry.RemoteAddress, entry.RemotePort, entry.State)

	// Handle RST, immediately terminate connection for normal sockets
	if header.Flags&TCP_RST != 0 && entry.State != TCP_LISTEN {
		handleRST(ts, entry)
		return nil
	}

	//fmt.Println("Handling packet in state: ", entry.State)

	switch entry.State {

	// Handshake
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

	// Established
	case TCP_ESTABLISHED:
		if header.Flags&TCP_ACK != 0 || len(payload) > 0 {
			handleEstablishedPacket(ts, entry, header, payload)
		}
		if header.Flags&TCP_FIN != 0 {
			handleFIN(ts, entry, header)
		}

	// Tear down
	case TCP_FIN_WAIT_1:
		// if header.Flags&TCP_FIN != 0 {
		// 	handleFIN(ts, entry, header)
		// } This would be a simultaneous close, which we don't support
		if len(payload) > 0 {
			handleEstablishedPacket(ts, entry, header, payload)
		}
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}
	case TCP_FIN_WAIT_2:
		// if header.Flags&TCP_ACK != 0 {
		// 	handleEstablishedACK(ts, entry, header) // We may need this for retransmissions, but we should've sent all other packets already
		// }
		if len(payload) > 0 {
			handleEstablishedPacket(ts, entry, header, payload)
		}
		if header.Flags&TCP_FIN != 0 {
			handleFIN(ts, entry, header)
		}
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}

	case TCP_CLOSE_WAIT:
		// if header.Flags&TCP_FIN != 0 {
		// 	handleFIN(ts, entry, header)
		// }
		// Should do nothing here

	case TCP_CLOSING:
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}

	case TCP_TIME_WAIT:
		if header.Flags&TCP_FIN != 0 {
			handleFIN(ts, entry, header)
		}
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}

	case TCP_LAST_ACK:
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}

	case TCP_CLOSED:
		// Do nothing
	}

	return nil
}

// Handshake functions
func handleSYN(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, srcAddr netip.Addr) {
	// Create new connection in SYN_RECEIVED state
	newSocket := &NormalSocket{
		SID:           ts.generateSID(),
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
		buf:             ringbuffer.New(int(BUFFER_SIZE)),
		ISS:             newSocket.SeqNum,
		UNA:             newSocket.SeqNum,
		NXT:             newSocket.SeqNum + 1, // +1 for SYN
		WND:             header.WindowSize,
		RTOtimer:        time.NewTimer(MIN_RTO), // This is the default value
		calculatedRTO:   MIN_RTO,
		SRTT:            0,
		RTTVAR:          0,
		retransmissions: 0,
	}
	newSocket.snd.RTOtimer.Stop()
	newSocket.snd.buf.SetBlocking(true)

	newSocket.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		IRS: header.SeqNum,
		NXT: header.SeqNum + 1, // +1 for SYN
		WND: BUFFER_SIZE,
	}
	newSocket.rcv.buf.SetBlocking(true)

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
		WindowSize: newSocket.rcv.WND, // Advertise our receive window
	}


	// Add it to inflight packets
	newSocket.snd.inFlightPackets.mutex.Lock()
	newSocket.snd.inFlightPackets.packets = append(newSocket.snd.inFlightPackets.packets, InFlightPacket{
		SeqNum:      synAckHeader.SeqNum,
		Length:      0,
		timeSent:    time.Now(),
		flags:       synAckHeader.Flags,
		windowFlags: synAckHeader.WindowSize,
	})
	newSocket.snd.inFlightPackets.mutex.Unlock()

	// Reset retransmission timer
	newSocket.snd.RTOtimer.Reset(newSocket.snd.calculatedRTO)

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
	socket.rcv.NXT = header.SeqNum + 1 // +1 for SYN

	// Update send state
	socket.snd.UNA = header.AckNum
	socket.snd.NXT = header.AckNum
	socket.snd.WND = header.WindowSize // Store peer's advertised window

	entry.State = TCP_ESTABLISHED

	// Signal that connection is established
	if socket.establishedChan != nil {
		close(socket.establishedChan)
		socket.establishedChan = nil
	}

	// Send ACK
	ackHeader := &TCPHeader{
		SourcePort: entry.LocalPort,
		DestPort:   entry.RemotePort,
		SeqNum:     socket.snd.NXT,
		AckNum:     socket.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_ACK,
		WindowSize: socket.rcv.WND, // Advertise our receive window
	}

	// We just set inflightpackets to empty and stop the timer here because we don't need to track the SYN-ACK, since it won't be acknowledged
	socket.snd.RTOtimer.Stop()
	socket.snd.inFlightPackets.mutex.Lock()
	socket.snd.inFlightPackets.packets = make([]InFlightPacket, 0)
	socket.snd.inFlightPackets.mutex.Unlock()

	packet := serializeTCPPacket(ackHeader, nil)
	ts.sendPacket(entry.RemoteAddress, packet)
}
func handleACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)

	entry.State = TCP_ESTABLISHED
	socket.SeqNum++
	socket.AckNum = header.SeqNum + 1

	// If this is from a listening socket's child, add to accept queue
	parentEntry, err := ts.VFindTableEntry(entry.LocalAddress, entry.LocalPort, netip.Addr{}, 0)
	if err == nil && parentEntry.State == TCP_LISTEN {
		listenSocket := parentEntry.SocketStruct.(*ListenSocket)
		listenSocket.acceptQueue <- socket
	}

	// Clean up in-flight packets
	cleanUpInFlightPackets(socket, header)

	// Reset retransmission timer if we have unacked data
	if len(socket.snd.inFlightPackets.packets) > 0 {
		socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
	} else {
		socket.snd.RTOtimer.Stop()
	}

	fmt.Printf("Connection established with %s:%d\n", entry.RemoteAddress, entry.RemotePort)
}

// Established functions
func handleData(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, payload []byte) {
	socket := entry.SocketStruct.(*NormalSocket)

	// fmt.Println("Data received")
	// fmt.Println("Header seq num: ", header.SeqNum)
	// fmt.Println("Socket rcv nxt: ", socket.rcv.NXT)

	if header.SeqNum == socket.rcv.NXT {
		// Next expected sequence number matches, process in order data
		processInOrderData(socket, payload)
	} else if header.SeqNum != socket.rcv.NXT { // != so that we take into account repeated data, which will be the case for retransmissions and ZWP
		//fmt.Println("Out of order packet received")
		// Store out of order data
		socket.rcv.earlyData = append(socket.rcv.earlyData, EarlyData{
			data:   payload,
			SeqNum: header.SeqNum,
			Length: uint16(len(payload)),
		})

		// Send duplicate ACK for the last in order se received
		ackHeader := &TCPHeader{
			SourcePort: entry.LocalPort,
			DestPort:   entry.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT, // next expected seq num
			DataOffset: 5,
			Flags:      TCP_ACK,
			WindowSize: socket.rcv.WND,
		}

		packet := serializeTCPPacket(ackHeader, nil)
		ts.sendPacket(entry.RemoteAddress, packet)
	}
}

func processInOrderData(socket *NormalSocket, payload []byte) {
	n, err := socket.rcv.buf.Write(payload)
	if err != nil {
		fmt.Printf("Error writing to receive buffer: %v\n", err)
		return
	}

	socket.rcv.NXT += uint32(n)

	// Iterate as long as we find packets that are now in order
	for {
		nextPacket := findNextSequence(socket.rcv.earlyData, socket.rcv.NXT)
		if nextPacket == nil {
			break
		}

		n, err := socket.rcv.buf.Write(nextPacket.data)
		if err != nil {
			fmt.Printf("Error writing early data to receive buffer: %v\n", err)
			break
		}

		socket.rcv.NXT += uint32(n)

		socket.rcv.earlyData = removePacket(socket.rcv.earlyData, nextPacket.SeqNum)
	}

	socket.rcv.WND = uint16(socket.rcv.buf.Free())

	// Send ACK for all processed data
	// Per RFC spec, sending ACK for last in order byte received
	ackHeader := &TCPHeader{
		SourcePort: socket.LocalPort,
		DestPort:   socket.RemotePort,
		SeqNum:     socket.snd.NXT,
		AckNum:     socket.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_ACK,
		WindowSize: socket.rcv.WND,
	}

	packet := serializeTCPPacket(ackHeader, nil)
	socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
}

func findNextSequence(earlyData []EarlyData, expectedSeq uint32) *EarlyData {
	for i := range earlyData {
		if earlyData[i].SeqNum == expectedSeq {
			return &earlyData[i]
		}
	}
	return nil
}

func removePacket(earlyData []EarlyData, seqNum uint32) []EarlyData {
	for i := range earlyData {
		if earlyData[i].SeqNum == seqNum {
			return append(earlyData[:i], earlyData[i+1:]...)
		}
	}
	return earlyData
}

func handleEstablishedPacket(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, payload []byte) {
	socket := entry.SocketStruct.(*NormalSocket)

	// Update send window
	socket.snd.WND = header.WindowSize

	// 1. Process any data
	if len(payload) > 0 {
		handleData(ts, entry, header, payload)
	}

	// 2. Process ACK if present and there are in flight packets
	if header.Flags&TCP_ACK != 0 && len(socket.snd.inFlightPackets.packets) > 0 {
		// Ignore old ACKs
		if header.AckNum <= socket.snd.UNA {
			return
		}

		socket.snd.UNA = header.AckNum

		// Clean up in-flight packets
		cleanUpInFlightPackets(socket, header)

		// Reset retransmission timer if we have unacked data
		if len(socket.snd.inFlightPackets.packets) > 0 {
			socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
		} else {
			socket.snd.RTOtimer.Stop()
		}

		// Reset retransmission count on successful ACK
		socket.snd.retransmissions = 0
	}
}

func cleanUpInFlightPackets(socket *NormalSocket, header *TCPHeader) {
	// Remove acknowledged packets from in-flight list
	oldUNA := socket.snd.UNA
	socket.snd.inFlightPackets.mutex.Lock()
	newPackets := make([]InFlightPacket, 0)
	for _, pkt := range socket.snd.inFlightPackets.packets {
		if pkt.SeqNum+uint32(pkt.Length) > header.AckNum {
			newPackets = append(newPackets, pkt)
		} else {
			// Use acknowledged packet for RTT calculation
			if pkt.SeqNum == oldUNA {
				socket.computeRTO(pkt.SeqNum, pkt.timeSent)
			}
		}
	}
	socket.snd.inFlightPackets.packets = newPackets
	socket.snd.inFlightPackets.mutex.Unlock()
}

func handleFIN(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)

	// Update socket state
	socket.rcv.NXT = header.SeqNum + 1
	socket.rcv.WND = header.WindowSize

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

	cleanUpInFlightPackets(socket, header)

	// Update state depending on current state
	if entry.State == TCP_ESTABLISHED {
		entry.State = TCP_CLOSE_WAIT
	}

	if entry.State == TCP_FIN_WAIT_2 {
		entry.State = TCP_TIME_WAIT
		// Now wait for 2 * MSL before transitioning to CLOSED
		// time.Sleep(2 * MSL) This doesn't work obviously
		go func() {
			time.Sleep(2 * MSL)
			entry.State = TCP_CLOSED
			ts.VDeleteTableEntry(*entry)
		}()
	}

	// If we're in TIME_WAIT, we just resend the ACK but don't change the state
}

func handleClosingACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	// Handle ACKs for four way closing handshake
	socket := entry.SocketStruct.(*NormalSocket)

	// Update socket state
	// socket.rcv.NXT = header.SeqNum + 1
	socket.rcv.WND = header.WindowSize

	// Clean up in-flight packets
	cleanUpInFlightPackets(socket, header)

	// We stop the timer since we're done sending data, and no further ACKs are expected
	socket.snd.RTOtimer.Stop()

	// Update state depending on current state
	if entry.State == TCP_FIN_WAIT_1 {
		entry.State = TCP_FIN_WAIT_2
	}
	// We won't handle closing because that's for simultaneous close, which we don't support
	if entry.State == TCP_LAST_ACK {
		entry.State = TCP_CLOSED
		// Remove the TCB
		ts.VDeleteTableEntry(*entry)
	}
}

func handleRST(ts *TCPStack, entry *TCPTableEntry) {
	// Immediately terminate connection, skip close state machine
	entry.State = TCP_CLOSED
	ts.VDeleteTableEntry(*entry)
}

// Event                    State (A)   State (B)
// A sends FIN              FIN_WAIT_1  ESTABLISHED
// B sends ACK              FIN_WAIT_2  CLOSE_WAIT
// B sends FIN              FIN_WAIT_2  LAST_ACK
// A sends ACK              TIME_WAIT   CLOSED
// A waits in TIME_WAIT     TIME_WAIT   CLOSED
// A transitions to CLOSED  CLOSED      CLOSED

// Other state changes:
// After sending first FIN: A goes from ESTABLISHED to FIN_WAIT_1
// After sending FIN in CLOSE_WAIT: A goes to LAST_ACK
// After waiting for 2*MSL in TIME_WAIT: A goes to CLOSED
