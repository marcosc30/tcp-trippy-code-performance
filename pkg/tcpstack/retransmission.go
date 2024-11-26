package tcpstack

import (
	"fmt"
	"sync"
	"time"
)

// Here, we will define our logic for retransmission of packets, including RTO calculations

type InFlightPacket struct {
	data     []byte // This may be too much overhead to track the data of every in flight packet
	SeqNum   uint32
	Length   uint16
	timeSent time.Time
	flags    uint8
	//CalculatedRTO time.Duration // This should be done per connection, not per packet
}

type InFlightPacketStack struct {
	packets []InFlightPacket
	mutex   sync.Mutex
}

func (socket *NormalSocket) manageRetransmissions() {
	// This function should be running on a separate goroutine, checks for RTO timer expiration and then retransmits packets
	for {
		select {
		case <-socket.snd.RTOtimer.C:
			fmt.Println("Retransmitting packet")

			// RTO timer expired
			err := socket.retransmitPacket()
			if err != nil {
				// We should close the connection here
				fmt.Println("Error retransmitting packet: ", err)
				socket.VClose()
			}
		}
	}
}

func (socket *NormalSocket) retransmitPacket() error {
	// Should be called whenever RTO timer expires

	socket.snd.inFlightPackets.mutex.Lock()
	defer socket.snd.inFlightPackets.mutex.Unlock()
	inflightpackets := socket.snd.inFlightPackets.packets

	for _, packet := range inflightpackets {
		if packet.SeqNum >= socket.snd.UNA {
			// This is the first unacked segment

			// Check if we have reached max retransmissions
			if socket.snd.retransmissions > TCP_RETRIES {
				return fmt.Errorf("max retransmissions reached")
			}

			// Create header for retransmission
			header := &TCPHeader{
				SourcePort: socket.LocalPort,
				DestPort:   socket.RemotePort,
				SeqNum:     packet.SeqNum,
				AckNum:     socket.rcv.NXT,
				DataOffset: 5,
				Flags:      packet.flags,
				WindowSize: uint16(socket.rcv.buf.Free()),
			}

			// Retransmit the packet
			tcpPacket := serializeTCPPacket(header, packet.data)
			if err := socket.tcpStack.sendPacket(socket.RemoteAddress, tcpPacket); err != nil {
				return err
			}

			// Increment retransmissions
			socket.snd.retransmissions++

			// Double the RTO timer
			socket.snd.calculatedRTO *= 2
			// Enforce maximum RTO
			if socket.snd.calculatedRTO > 60*time.Second {
				socket.snd.calculatedRTO = 60 * time.Second
			}
			socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)

			//socket.snd.inFlightPackets.packets = inflightpackets[i:]

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
	socket.snd.inFlightPackets.mutex.Lock()
	defer socket.snd.inFlightPackets.mutex.Unlock()
	for _, packet := range socket.snd.inFlightPackets.packets {
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
		socket.snd.RTTVAR = time.Duration(float64(socket.snd.RTTVAR.Nanoseconds())*(1-beta)+float64(abs(int64(socket.snd.SRTT-rtt)))*beta) * time.Nanosecond
		socket.snd.SRTT = time.Duration((1-alpha)*float64(socket.snd.SRTT.Nanoseconds())+alpha*float64(rtt.Nanoseconds())) * time.Nanosecond
	}

	// Calculate RTO
	socket.snd.calculatedRTO = socket.snd.SRTT + 4*socket.snd.RTTVAR

	// Enforce minimum and maximum bounds
	if socket.snd.calculatedRTO < time.Second {
		socket.snd.calculatedRTO = time.Second
	} else if socket.snd.calculatedRTO > MSL {
		socket.snd.calculatedRTO = MSL
	}

	// // Reset the RTO timer
	// socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)
	// I don't think this last bit is needed since we should reset it separately whenever we compute RTO in functions that do it
	// But given that timers should reset on recalculation, it may be smart to reset it within this function

}
