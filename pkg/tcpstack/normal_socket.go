package tcpstack

import (
	"fmt"
	"io"
	"net/netip"
	"os"
	"time"
	"github.com/smallnest/ringbuffer"
)

func (ns *NormalSocket) GetSID() int {
	return ns.SID
}

func (ns *NormalSocket) VClose() error {
	// Check if connection is in the right state
	table_entry, _ := ns.tcpStack.VFindTableEntry(ns.LocalAddress, ns.LocalPort, ns.RemoteAddress, ns.RemotePort)
	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_CLOSE_WAIT {
		return fmt.Errorf("connection not established")
	}

	// Send FIN packet
	fmt.Println("ack num: ", ns.rcv.NXT)
	header := &TCPHeader{
		SourcePort: ns.LocalPort,
		DestPort:   ns.RemotePort,
		SeqNum:     ns.SeqNum,
		AckNum:     ns.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_FIN,
		WindowSize: ns.rcv.WND,
	}

	// Manage the RTO timer (if there are inflightpackets, it is running and we leave it, if not, we reset)
	if len(ns.snd.inFlightPackets.packets) == 0 {
		ns.snd.RTOtimer.Reset(ns.snd.calculatedRTO)
	}

	//fmt.Print("vclose locking packets mutex")
	ns.snd.inFlightPackets.mutex.Lock()
	ns.snd.inFlightPackets.packets = append(ns.snd.inFlightPackets.packets, InFlightPacket{
		data:     nil,
		SeqNum:   ns.SeqNum,
		Length:   0,
		timeSent: time.Now(),
		flags:    TCP_FIN,
	})
	ns.snd.inFlightPackets.mutex.Unlock()
	//fmt.Println("unlocked packets mutex")

	packet := serializeTCPPacket(header, nil)
	err := ns.tcpStack.sendPacket(ns.RemoteAddress, packet)
	if err != nil {
		return err
	}

	// Update state to FIN_WAIT_1
	table_entry, _ = ns.tcpStack.VFindTableEntry(ns.LocalAddress, ns.LocalPort, ns.RemoteAddress, ns.RemotePort)
	if table_entry.State == TCP_ESTABLISHED {
		table_entry.State = TCP_FIN_WAIT_1
	} else if table_entry.State == TCP_CLOSE_WAIT {
		table_entry.State = TCP_LAST_ACK
	}

	return nil
}

func (ns *NormalSocket) VConnect(tcpStack *TCPStack, remoteAddress netip.Addr, remotePort uint16) error {
	fmt.Println("Connecting to: ", remoteAddress, remotePort)
	// Add a connection established channel
	connEstablished := make(chan struct{})
	
	ns.SID = tcpStack.generateSID()
	ns.tcpStack = tcpStack
	ns.RemoteAddress = remoteAddress
	ns.RemotePort = remotePort
	ns.LocalPort = tcpStack.allocatePort()
	ns.SeqNum = generateInitialSeqNum()
	ns.lastActive = time.Now()
	ns.establishedChan = connEstablished  // Store the channel in the socket

	// Initialize send/receive state
	ns.snd = SND{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		ISS: ns.SeqNum,
		UNA: ns.SeqNum,
		NXT: ns.SeqNum + 1, // +1 for SYN
		WND: BUFFER_SIZE,
		//RTOtimer:      time.NewTimer(1 * time.Second), // This is the default value
		calculatedRTO:   1 * time.Second,
		SRTT:            0,
		RTTVAR:          0,
		retransmissions: 0,
	}
	ns.snd.buf.SetBlocking(true)
	ns.snd.RTOtimer = time.NewTimer(ns.snd.calculatedRTO)
	ns.snd.RTOtimer.Stop()

	ns.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		WND: BUFFER_SIZE,
	}
	ns.rcv.buf.SetBlocking(true)

	// Set local address to first interface address (for now)
	for _, iface := range tcpStack.ipStack.Interfaces {
		ns.LocalAddress = iface.IPAddr
		break
	}

	// Create new TCP table entry
	entry := TCPTableEntry{
		LocalAddress:  ns.LocalAddress,
		LocalPort:     ns.LocalPort,
		RemoteAddress: remoteAddress,
		RemotePort:    remotePort,
		State:         TCP_SYN_SENT,
		SocketStruct:  ns,
	}

	tcpStack.VInsertTableEntry(entry)

	// Send SYN packet with our window size
	header := &TCPHeader{
		SourcePort: ns.LocalPort,
		DestPort:   remotePort,
		SeqNum:     ns.SeqNum,
		DataOffset: 5,
		Flags:      TCP_SYN,
		WindowSize: ns.rcv.WND,
	}

	// Add to in-flight packets
	ns.snd.inFlightPackets.mutex.Lock()
	ns.snd.inFlightPackets.packets = append(ns.snd.inFlightPackets.packets, InFlightPacket{
		data:     nil,
		SeqNum:   ns.SeqNum,
		Length:   0,
		timeSent: time.Now(),
		flags:    TCP_SYN,
	})
	ns.snd.inFlightPackets.mutex.Unlock()

	// Reset RTO timer (always the first packet so we know to start it here)
	ns.snd.RTOtimer.Reset(ns.snd.calculatedRTO)

	packet := serializeTCPPacket(header, nil)
	err := tcpStack.sendPacket(remoteAddress, packet)
	if err != nil {
		fmt.Println("Error sending SYN packet: ", err)
		return err
	}

	// Wait for connection to be established or timeout
	select {
	case <-connEstablished:
		return nil
	case <-time.After(HANDSHAKE_TIMEOUT): 
		// Remove socket entry
		tcpStack.VDeleteTableEntry(entry)
		return fmt.Errorf("connection timeout")
	}
}

func (socket *NormalSocket) VWrite(data []byte) error {
	// Check if connection is in the right state

	table_entry, err := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
	if err != nil {
		fmt.Println("Error finding table entry: ", err)
		return err
	}

	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_CLOSE_WAIT {
		return fmt.Errorf("connection not established")
	}

	// Write data to send buffer, break into max size of space left in buffer and continue to send until all sent

	for len(data) > 0 {
		// Write data to send buffer
		n, err := socket.snd.buf.TryWrite(data)
		if err != nil && err != ringbuffer.ErrIsFull && err != ringbuffer.ErrTooMuchDataToWrite {
			return err
		}

		// Try to send data
		err = socket.trySendData()
		if err != nil {
			return err
		}

		// Update data to send
		data = data[n:]
	}

	return nil
}

// Send RST packet
func (socket *NormalSocket) sendRST() error {
	header := &TCPHeader{
		SourcePort: socket.LocalPort,
		DestPort:   socket.RemotePort,
		SeqNum:     socket.snd.NXT,
		AckNum:     socket.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_RST,
	}

	packet := serializeTCPPacket(header, nil)
	return socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
}

func (socket *NormalSocket) trySendData() error {
	// Changed this to a for loop to send larger packets, may need to revise this
	for socket.snd.buf.Length() > 0 {
		// bufferSpace := socket.snd.buf.Length()

		// Free window space is size of receiver buffer - amount of data in flight
		// Calculate data in flight using pointers
		dataInFlight := socket.snd.NXT - socket.snd.UNA
		freeWindowSpace := socket.snd.WND - uint16(dataInFlight)

		//fmt.Println("In flight packets: ", len(socket.snd.inFlightPackets))
		// if bufferSpace == 0{ //&& //len(socket.snd.inFlightPackets) == 0 { this is not needed, since retransmissions should be handled separately in a go routine
		// 	// But the problem of not reading acks while trying to send data is bad because if we have a lot in our buffer we won't be able to read acks until we're done
		// 	// Which is not good and will lead to a lot of retransmissions


		if socket.snd.WND > 0 {
			if freeWindowSpace <= 0 {
				continue
				// return nil
			}
			maxSendSize := min(int(freeWindowSpace), MAX_TCP_PAYLOAD)

			sendData := make([]byte, maxSendSize)
			//  socket.snd.buf.SetBlocking(true) // We don't want blocking here, since we should never be trying to send more than the buffer has
			n, err := socket.snd.buf.Read(sendData)
			// This will return an error if there's nothing in the buffer
			if err != nil {
				return err
			}

			header := &TCPHeader{
				SourcePort: socket.LocalPort,
				DestPort:   socket.RemotePort,
				SeqNum:     socket.snd.NXT,
				AckNum:     socket.rcv.NXT,
				DataOffset: 5,
				Flags:      TCP_ACK,
				WindowSize: uint16(socket.rcv.buf.Free()), // Our current receive window
				Checksum:   0,
			}


			// Send data packet
			packet := serializeTCPPacket(header, sendData[:n])
			err = socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
			if err != nil {
				return err
			}
			
			// We figure out if inflight packets is empty here to know if we should reset the RTO timer
			if len(socket.snd.inFlightPackets.packets) == 0 {
				socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
			}

			// Add it to the inflight packets
			socket.snd.inFlightPackets.mutex.Lock()
			socket.snd.inFlightPackets.packets = append(socket.snd.inFlightPackets.packets, InFlightPacket{
				data:     sendData[:n],
				SeqNum:   socket.snd.NXT,
				Length:   uint16(n),
				timeSent: time.Now(),
				flags:    TCP_ACK,
			})
			socket.snd.inFlightPackets.mutex.Unlock()

			// Update send buffer sequence number
			socket.snd.NXT += uint32(n)

		} else if socket.snd.WND == 0 {
			// Send zero window probe
			err := socket.sendZeroWindowProbe()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// New helper function for zero window probing
func (socket *NormalSocket) sendZeroWindowProbe() error {
	// Keep probing until either we get a non-zero window or hit retry limit
	retries := 0
	next_byte_data := make([]byte, 1)
	socket.snd.buf.TryRead(next_byte_data) // Should always have data or we shouldn't be here

	for socket.snd.WND == 0 && retries < ZWP_RETRIES {
		fmt.Println("Sending zero window probe")
		fmt.Println("WND: ", socket.snd.WND)
		header := &TCPHeader{
			SourcePort: socket.LocalPort,
			DestPort:   socket.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT,
			DataOffset: 5,
			Flags:      TCP_ACK,
			WindowSize: uint16(socket.rcv.buf.Free()),
		}


		// We read the byte from the buffer to send it
		packet := serializeTCPPacket(header, next_byte_data)
		err := socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
		if err != nil {
			return err
		}

		// socket.snd.NXT += 1
		retries++

		// Wait for response before sending next probe
		time.Sleep(ZWP_PROBE_INTERVAL)
	}

	if retries >= ZWP_RETRIES {
		return fmt.Errorf("zero window probe max retries exceeded")
	}

	socket.snd.NXT++
	fmt.Println("Window size: ", socket.snd.WND)

	return nil
}

// New helper function for sending data packets
func (socket *NormalSocket) sendDataPacket(data []byte) error {
	header := &TCPHeader{
		SourcePort: socket.LocalPort,
		DestPort:   socket.RemotePort,
		SeqNum:     socket.snd.NXT,
		AckNum:     socket.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_ACK,
		WindowSize: uint16(socket.rcv.buf.Free()),
	}

	fmt.Println("Seq num: ", socket.snd.NXT)

	packet := serializeTCPPacket(header, data)
	err := socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
	if err != nil {
		return err
	}

	// Track in-flight packet
	socket.snd.inFlightPackets.mutex.Lock()
	socket.snd.inFlightPackets.packets = append(socket.snd.inFlightPackets.packets, InFlightPacket{
		data:     data,
		SeqNum:   socket.snd.NXT,
		Length:   uint16(len(data)),
		timeSent: time.Now(),
		flags:    TCP_ACK,
	})
	socket.snd.inFlightPackets.mutex.Unlock()

	socket.snd.NXT += uint32(len(data))
	return nil
}

// Why is there no min already...
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// No abs either oof
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func (socket *NormalSocket) VRead(data []byte) (int, error) {
	// Check if connection is in the right state
	table_entry, _ := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_FIN_WAIT_1 && table_entry.State != TCP_FIN_WAIT_2 {
		return 0, fmt.Errorf("connection not established")
	}

	// Read data from receive buffer
	n, err := socket.rcv.buf.Read(data)
	if err != nil {
		return 0, err
	}

	// Update window
	socket.rcv.WND = uint16(socket.rcv.buf.Free())

	return n, nil
}

func (socket *NormalSocket) VSendFile(filename string) error {
	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Println("Sending file")

	// Track total bytes sent
	var totalBytesSent int64 = 0

	// Read file into buffer
	buffer := make([]byte, BUFFER_SIZE)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			// Only return if it's an error other than EOF
			fmt.Println("Error reading file: ", err)
			return err
		}
		totalBytesSent += int64(n)

		if n > 0 {
			// Write what we actually read
			err = socket.VWrite(buffer[:n])
			if err != nil {
				return err
			}
		}

		// Break if we've reached the end of the file
		if err == io.EOF {
			break
		}
	}

	fmt.Printf("Sent %d total bytes\n", totalBytesSent)
	fmt.Println("Closing connection")
	
	// Close the connection after sending the file
	err = socket.VClose()
	if err != nil {
		return fmt.Errorf("error closing connection after file transfer: %v", err)
	}

	return nil
}

func (socket *NormalSocket) VReceiveFile(filename string) error {
	// Open file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Track total bytes received
	var totalBytesReceived int64 = 0

	buffer := make([]byte, BUFFER_SIZE)
	for {
		// Check if connection is closed before each read
		table_entry, err := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
		if err != nil {
			return err
		}

		// If we're in CLOSE_WAIT state, it means we've received FIN from the sender
		if table_entry.State == TCP_CLOSE_WAIT {
			break
		}

		n, err := socket.VRead(buffer)
		if err != nil {
			// Only return if it's not a connection closing error
			if err.Error() != "connection not established" {
				return err
			}
			break
		}
		totalBytesReceived += int64(n)

		if n > 0 {
			_, err = file.Write(buffer[:n])
			if err != nil {
				return err
			}
		}
	}

	fmt.Printf("Received %d total bytes\n", totalBytesReceived)
	fmt.Println("Closing connection")

	// Wait until you are in the right state so that there is no simultaneous close which is not implemented
	
	for {
		table_entry, err := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
		if err != nil {
			return err
		}

		if table_entry.State == TCP_CLOSE_WAIT {
			break
		}
	}

	err = socket.VClose()
	if err != nil {
		return fmt.Errorf("error closing connection after file transfer: %v", err)
	}

	return nil
}
