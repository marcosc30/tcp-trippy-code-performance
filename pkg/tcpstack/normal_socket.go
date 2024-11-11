package tcpstack

import (
	"errors"
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
}

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
	ns.LocalPort = tcpStack.allocateEphemeralPort()
	ns.SeqNum = generateInitialSeqNum()

	// Initialize send/receive state
	ns.snd = SND{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		ISS: ns.SeqNum,
		UNA: ns.SeqNum,
		NXT: ns.SeqNum + 1, // +1 for SYN
		WND: BUFFER_SIZE,
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
	// Write data to send buffer
	n, err := socket.snd.buf.Write(data)
	if err != nil {
		return err
	}

	// Max data to send (minimum of receiver's window and data available)
	availableData := socket.snd.buf.Length()
	maxSendSize := min(int(socket.rcv.WND), availableData)

	// If we can send data, create and send TCP segments
	if maxSendSize > 0 {
		// Read from send buffer
		sendData := make([]byte, maxSendSize)
		n, err = socket.snd.buf.Read(sendData)
		if err != nil {
			return err
		}

		// Create TCP header
		header := &TCPHeader{
			SourcePort: socket.LocalPort,
			DestPort:   socket.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT,
			DataOffset: 5,
			Flags:      TCP_ACK | TCP_PSH, // PSH flag to deliver data immediately
			WindowSize: socket.rcv.WND,
		}

		packet := serializeTCPPacket(header, sendData[:n])
		err = socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
		if err != nil {
			return err
		}

		socket.snd.NXT += uint32(n)
	} else if maxSendSize == 0 {
		// Here, we implement 0 window probing

		// We start a timer to decide on when to timeout
		timeout := 5 * time.Second
		timer := time.NewTimer(timeout)

		for {
			select {
			// This may be better to implement with the writeReady channel, also check that the window being checked is the right one
			case <-timer.C:
				// Timeout
				timer.Stop()
				return errors.New("Timeout, no response from receiver")
			default:
				if socket.snd.WND > 0 {
					// Receiver window is now open
					return socket.VWrite(data)
				}

				// Send an empty packet with ACK flag
				header := &TCPHeader{
					SourcePort: socket.LocalPort,
					DestPort:   socket.RemotePort,
					SeqNum:     socket.snd.NXT,
					AckNum:     socket.rcv.NXT,
					DataOffset: 1,
					Flags:      TCP_ACK,
					WindowSize: socket.rcv.WND,
				}

				packet := serializeTCPPacket(header, nil)
				err = socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
				if err != nil {
					return err
				}
			}	
		}
	}

	return nil
}

// Helper function for uint16 min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (socket *NormalSocket) VRead(data []byte) (int, error) {
	// Wait for data to be available
	n, err := socket.rcv.buf.Read(data)
	if err != nil {
		return 0, err
	}

	// Update receive window after reading data
	socket.rcv.WND = uint16(socket.rcv.buf.Free())

	// Send window update if needed
	if socket.rcv.WND >= BUFFER_SIZE/2 {
		header := &TCPHeader{
			SourcePort: socket.LocalPort,
			DestPort:   socket.RemotePort,
			SeqNum:     socket.snd.NXT,
			AckNum:     socket.rcv.NXT,
			DataOffset: 5,
			Flags:      TCP_ACK,
			WindowSize: socket.rcv.WND,
		}

		packet := serializeTCPPacket(header, nil)
		socket.tcpStack.sendPacket(socket.RemoteAddress, packet)
	}

	return n, nil
}
