package tcpstack

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/smallnest/ringbuffer"
)

const MSL = 60 * time.Second

func (ts *TCPStack) HandlePacket(srcAddr, dstAddr netip.Addr, packet []byte) error {
	header, payload := ParseTCPHeader(packet)

	entry, err := ts.VFindTableEntry(dstAddr, header.DestPort, srcAddr, header.SourcePort)
	if err != nil {
		return err
	}

	//fmt.Println("Handling packet for", entry.LocalAddress, entry.LocalPort, entry.RemoteAddress, entry.RemotePort, entry.State)

	switch entry.State {

	// Handshake
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

	// Established
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

	// Tear down
	case TCP_FIN_WAIT_1:
		// if header.Flags&TCP_FIN != 0 {
		// 	handleFIN(ts, entry, header)
		// } This would be a simultaneous close, which we don't support
		if len(payload) > 0 {
			handleData(ts, entry, header, payload)
		}
		if header.Flags&TCP_ACK != 0 {
			handleClosingACK(ts, entry, header)
		}
	case TCP_FIN_WAIT_2:
		// if header.Flags&TCP_ACK != 0 {
		// 	handleEstablishedACK(ts, entry, header) // We may need this for retransmissions, but we should've sent all other packets already
		// }
		if len(payload) > 0 {
			handleData(ts, entry, header, payload)
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
		WND:             BUFFER_SIZE,
		RTOtimer:        time.NewTimer(1 * time.Second), // This is the default value
		calculatedRTO:   1 * time.Second,
		SRTT:            0,
		RTTVAR:          0,
		retransmissions: 0,
	}
	newSocket.snd.RTOtimer.Stop()

	newSocket.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		IRS: header.SeqNum,
		NXT: header.SeqNum + 1, // +1 for SYN
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
		WindowSize: newSocket.rcv.WND, // Advertise our receive window
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
	socket.rcv.NXT = header.SeqNum + 1 // +1 for SYN
	socket.rcv.WND = header.WindowSize // Store peer's advertised window

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
		WindowSize: socket.rcv.WND, // Advertise our receive window
	}

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

	fmt.Printf("Connection established with %s:%d\n", entry.RemoteAddress, entry.RemotePort)
}

// Established functions
func handleData(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader, payload []byte) {
	socket := entry.SocketStruct.(*NormalSocket)

	// Check if sequence number matches what we expect
	if header.SeqNum == socket.rcv.NXT {
		// Write the data to receive buffer
		n, err := socket.rcv.buf.Write(payload)
		// If bigger than the buffer, recieve enough to fill, then send back
		// ack for bytes written, allows sender to zero window probe
		if err != nil {
			fmt.Printf("Error writing to receive buffer: %v\n", err)
			return
		}

		// Update next expected sequence number
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
		// TODO: Handle out of order data
		fmt.Println("Out of order data")
	}
}

func handleEstablishedACK(ts *TCPStack, entry *TCPTableEntry, header *TCPHeader) {
	socket := entry.SocketStruct.(*NormalSocket)

	// We need to address the case that this is an ACK with data in it that we're receiving, we don't want to reset our UNA as a sender in this case

	// Check if this is an ACK for a packet we sent, we may be able to do this with inflight packets, checking if it's empty or if the ack matches a packet
	if header.AckNum > socket.snd.UNA {

		// Now, we recompute the RTO
		socket.computeRTO(header.AckNum, time.Now())
		socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
		socket.snd.retransmissions = 0

		socket.snd.UNA = header.AckNum
		socket.snd.WND = header.WindowSize

		socket.lastActive = time.Now()

		// Remove it from the inflight packets
		socket.snd.inFlightPackets.mutex.Lock()
		for i, packet := range socket.snd.inFlightPackets.packets {
			if packet.SeqNum+uint32(packet.Length) <= header.AckNum {
				socket.snd.inFlightPackets.packets = append(socket.snd.inFlightPackets.packets[:i], socket.snd.inFlightPackets.packets[i+1:]...)
			}
		}
		socket.snd.inFlightPackets.mutex.Unlock()

		//socket.trySendData() why are we trying to send data here?

	}

	// If this is the last ACK for data sent, we should stop the timer
	if header.AckNum == socket.snd.NXT {
		socket.snd.RTOtimer.Stop()
	}
}

// Tear down functions

// Event					State (A)	State (B)
// A sends FIN				FIN_WAIT_1	ESTABLISHED
// B sends ACK				FIN_WAIT_2	CLOSE_WAIT
// B sends FIN				FIN_WAIT_2	LAST_ACK
// A sends ACK				TIME_WAIT	CLOSED
// A waits in TIME_WAIT		TIME_WAIT	CLOSED
// A transitions to CLOSED	CLOSED		CLOSED

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
	socket.rcv.NXT = header.SeqNum + 1
	socket.rcv.WND = header.WindowSize

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

// Other state changes:
// After sending first FIN: A goes from ESTABLISHED to FIN_WAIT_1
// After sending FIN in CLOSE_WAIT: A goes to LAST_ACK
// After waiting for 2*MSL in TIME_WAIT: A goes to CLOSED
