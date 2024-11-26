package tcpstack

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/smallnest/ringbuffer"
)


type NormalSocket struct {
	SID           int
	LocalAddress  netip.Addr
	LocalPort     uint16
	RemoteAddress netip.Addr
	RemotePort    uint16
	SeqNum        uint32
	AckNum        uint32
	tcpStack      *TCPStack
	snd           SND
	rcv           RCV
	lastActive    time.Time
}

const TCP_RETRIES = 3
const ZWP_PROBE_INTERVAL = 1 * time.Second

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
	header := &TCPHeader{
		SourcePort: ns.LocalPort,
		DestPort:   ns.RemotePort,
		SeqNum:     ns.SeqNum,
		AckNum:     ns.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_FIN,
		WindowSize: ns.rcv.WND,
	}

	ns.snd.inFlightPackets.mutex.Lock()
	ns.snd.inFlightPackets.packets = append(ns.snd.inFlightPackets.packets, InFlightPacket{
		data:     nil,
		SeqNum:   ns.SeqNum,
		Length:   0,
		timeSent: time.Now(),
		flags:    TCP_FIN,
	})
	ns.snd.inFlightPackets.mutex.Unlock()

	ns.snd.RTOtimer.Reset(ns.snd.calculatedRTO)

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
	ns.SID = tcpStack.generateSID()
	ns.tcpStack = tcpStack
	ns.RemoteAddress = remoteAddress
	ns.RemotePort = remotePort
	ns.LocalPort = tcpStack.allocatePort()
	ns.SeqNum = generateInitialSeqNum()
	ns.lastActive = time.Now()

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
		WindowSize: ns.rcv.WND, // Advertise our receive window
	}

	packet := serializeTCPPacket(header, nil)
	return tcpStack.sendPacket(remoteAddress, packet)
}

func (socket *NormalSocket) VWrite(data []byte) error {
	fmt.Println("VWrite")

	// Check if connection is in the right state

	table_entry, err := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
	if err != nil {
		fmt.Println("Error finding table entry: ", err)
		return err
	}

	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_CLOSE_WAIT {
		return fmt.Errorf("connection not established")
	}

	// Write data to send buffer
	_, err = socket.snd.buf.Write(data)
	if err != nil {
		return err
	}

	// What is the behavior if there is more data than the send buffer can handle

	// Start the RTO timer
	socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)

	// We start the RTO timer here so that it is per write, not per packet

	// Try to send immediately
	return socket.trySendData()
}

func (socket *NormalSocket) trySendData() error {
	// Changed this to a for loop to send larger packets, may need to revise this
	for {
		// bufferSpace := socket.snd.buf.Length()

		// Free window space is size of receiver buffer - amount of data in flight
		// Calculate data in flight using pointers
		dataInFlight := socket.snd.NXT - socket.snd.UNA
		freeWindowSpace := socket.snd.WND - uint16(dataInFlight)
		fmt.Println("Free window space: ", freeWindowSpace)
		fmt.Println("WND: ", socket.snd.WND)
		fmt.Println("Data in flight: ", dataInFlight)


		//fmt.Println("In flight packets: ", len(socket.snd.inFlightPackets))
		// if bufferSpace == 0{ //&& //len(socket.snd.inFlightPackets) == 0 { this is not needed, since retransmissions should be handled separately in a go routine
		// 	// But the problem of not reading acks while trying to send data is bad because if we have a lot in our buffer we won't be able to read acks until we're done
		// 	// Which is not good and will lead to a lot of retransmissions

		// 	return nil
		// }

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
			}

			// Send data packet
			packet := serializeTCPPacket(header, sendData[:n])
			err = socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
			if err != nil {
				return err
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
			fmt.Println("Zero window, sending probe")
			// return nil
			// Here, we implement zero window probing
			// We send a probe packet to check if the window is still zero

			// We send one byte repeatedly

			// We don't add them to inflight because we don't want it to be retransmitted

			header := &TCPHeader{
				SourcePort: socket.LocalPort,
				DestPort:   socket.RemotePort,
				SeqNum:     socket.snd.NXT,
				AckNum:     socket.rcv.NXT,
				DataOffset: 5,
				Flags:      TCP_ACK,
				WindowSize: uint16(socket.rcv.buf.Free()), // Our current receive window
			}

			// Send data packet
			packet := serializeTCPPacket(header, []byte{0})
			err := socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
			if err != nil {
				return err
			}

			// Update send buffer sequence number
			socket.snd.NXT += 1

			if socket.snd.WND > 0 {
				break
			}

			// We sleep so that we don't send too many probes
			time.Sleep(ZWP_PROBE_INTERVAL)

			// We don't need to add a timer since we have the RTO timer
		}
	}
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

	// Optional, send window update
	// TODO (ask in milestone meeting): Should we send a window update?
	// I like this, but not implemented in reference

	// header := &TCPHeader{
	// 	SourcePort: socket.LocalPort,
	// 	DestPort:   socket.RemotePort,
	// 	SeqNum:     socket.snd.NXT,
	// 	AckNum:     socket.rcv.NXT,
	// 	DataOffset: 5,
	// 	Flags:      TCP_ACK,
	// 	WindowSize: uint16(socket.rcv.buf.Free()), // Current receive window
	// }

	// packet := serializeTCPPacket(header, nil)
	// socket.tcpStack.sendPacket(socket.RemoteAddress, packet)

	return n, nil
}
