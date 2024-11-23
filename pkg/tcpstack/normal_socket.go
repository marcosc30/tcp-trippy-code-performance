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

func (ns *NormalSocket) GetSID() int {
	return ns.SID
}

func (ns *NormalSocket) VClose() error {
	// TODO: Clean up TCP connection state
	return nil
}

func (ns *NormalSocket) VConnect(tcpStack *TCPStack, remoteAddress netip.Addr, remotePort uint16) error {
	ns.tcpStack = tcpStack
	ns.RemoteAddress = remoteAddress
	ns.RemotePort = remotePort
	ns.LocalPort = tcpStack.allocatePort()
	ns.SeqNum = generateInitialSeqNum()
	ns.lastActive = time.Now()

	// Initialize send/receive state
	ns.snd = SND{
		buf:           ringbuffer.New(int(BUFFER_SIZE)),
		ISS:           ns.SeqNum,
		UNA:           ns.SeqNum,
		NXT:           ns.SeqNum + 1, // +1 for SYN
		WND:           BUFFER_SIZE,
		RTOtimer:      time.NewTimer(1 * time.Second), // This is the default value
		calculatedRTO: 1 * time.Second,
		SRTT:          0,
		RTTVAR:        0,
	}

	ns.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		WND: BUFFER_SIZE,
	}

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
	// Write data to send buffer
	_, err := socket.snd.buf.Write(data)
	if err != nil {
		return err
	}

	// What is the behavior if there is more data than the send buffer can handle

	// Try to send immediately
	return socket.trySendData()
}

func (socket *NormalSocket) trySendData() error {
	// Changed this to a for loop to send larger packets, may need to revise this
	for {
		availableData := socket.snd.buf.Length()
		if availableData == 0 {
			return nil
		}
		maxSendSize := min(int(socket.snd.WND), availableData)
		// We should also add a maximum packet size restriction on this
		if availableData == 0 { // and make sure there are no inflight packets
			break
		}

		if maxSendSize > 0 {
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
			socket.snd.inFlightPackets = append(socket.snd.inFlightPackets, InFlightPacket{
				data:     sendData[:n],
				SeqNum:   socket.snd.NXT,
				Length:   uint16(n),
				timeSent: time.Now(),
			})

			// Update send buffer sequence number
			socket.snd.NXT += uint32(n)
		} else if maxSendSize == 0 {
			// Here, we implement zero window probing
			// We send a probe packet to check if the window is still zero

			// We send one byte repeatedly

			// We don't add them to inflight because we don't want it to be retransmitted
		}
	}

	return nil
}

func (socket *NormalSocket) manageRetransmissions() {
	// This function should be running on a separate goroutine, checks for RTO timer expiration and then retransmits packets
	for {
		select {
		case <-socket.snd.RTOtimer.C:
			// RTO timer expired
			err := socket.retransmitPacket()
			if err != nil {
				// We should close the connection here
				socket.VClose()
			}
		}
	}
}

func (socket *NormalSocket) retransmitPacket() error {
	// Should be called whenever RTO timer expires

	inflightpackets := socket.snd.inFlightPackets

	for i, packet := range inflightpackets {
		if packet.SeqNum >= socket.snd.UNA {
			// This is the first unacked segment

			// Check if we have reached max retransmissions
			if packet.Retransmissions > TCP_RETRIES {
				return fmt.Errorf("max retransmissions reached")
			}

			// Create header for retransmission
			header := &TCPHeader{
				SourcePort: socket.LocalPort,
				DestPort:   socket.RemotePort,
				SeqNum:     packet.SeqNum,
				AckNum:     socket.rcv.NXT,
				DataOffset: 5,
				Flags:      TCP_ACK,
				WindowSize: uint16(socket.rcv.buf.Free()),
			}

			// Retransmit the packet
			tcpPacket := serializeTCPPacket(header, packet.data)
			if err := socket.tcpStack.sendPacket(socket.RemoteAddress, tcpPacket); err != nil {
				return err
			}

			// Increment retransmissions
			packet.Retransmissions++

			// Double the RTO timer
			socket.snd.calculatedRTO *= 2
			socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)

			socket.snd.inFlightPackets = inflightpackets[i+1:]

			break

		}
	}

	return nil
}

func (socket *NormalSocket) computeRTO(ackNum uint32, timeReceived time.Time) {
	// We need to implement the RTO calculation here

	// Resets RTO calculation

	// Should use max retransmissions and RTT calculation to determine RTO

	// We need a way to track when packet was sent and when ack was received

	var rtt time.Duration

	// First we find the packet in the inflight packets using the ackNum, it shouldn't have been removed from in flights if it still hasn't been acked
	for _, packet := range socket.snd.inFlightPackets {
		if packet.SeqNum == ackNum {
			// Calculate the RTT
			rtt = timeReceived.Sub(packet.timeSent)

			break
		}
	}

	// If this is the first RTT measurement
	if socket.snd.SRTT == 0 {
		socket.snd.SRTT = rtt
		socket.snd.RTTVAR = rtt / 2
	} else {
		// Update RTTVAR and SRTT according to RFC 6298
		alpha := 0.125
		beta := 0.25
		socket.snd.RTTVAR = time.Duration(float64(socket.snd.RTTVAR.Nanoseconds())*(1-beta) + float64(abs(int64(socket.snd.SRTT-rtt)))*beta) * time.Nanosecond
		socket.snd.SRTT = time.Duration((1-alpha)*float64(socket.snd.SRTT.Nanoseconds()) + alpha*float64(rtt.Nanoseconds())) * time.Nanosecond
	}

	// Calculate RTO
	socket.snd.calculatedRTO = socket.snd.SRTT + 4*socket.snd.RTTVAR

	// Enforce minimum and maximum bounds
	if socket.snd.calculatedRTO < time.Second {
		socket.snd.calculatedRTO = time.Second
	} else if socket.snd.calculatedRTO > 60*time.Second {
		socket.snd.calculatedRTO = 60 * time.Second
	}

	// // Reset the RTO timer
	// socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
	// I don't think this last bit is needed since we should reset it separately whenever we compute RTO in functions that do it
	// But given that timers should reset on recalculation, it may be smart to reset it within this function

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
