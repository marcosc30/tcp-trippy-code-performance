package tcpstack

import (
	"fmt"
	"net/netip"
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
	ns.SID = tcpStack.generateSID()
	ns.tcpStack = tcpStack
	ns.RemoteAddress = remoteAddress
	ns.RemotePort = remotePort
	ns.LocalPort = tcpStack.allocatePort()
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
	fmt.Println("VWrite")
	// Write data to send buffer
	_, err := socket.snd.buf.Write(data)
	if err != nil {
		return err
	}

	// Try to send immediately
	return socket.trySendData()
}

func (socket *NormalSocket) trySendData() error {
	availableData := socket.snd.buf.Length()
	if availableData == 0 {
		return nil
	}

	maxSendSize := min(int(socket.snd.WND), availableData)
	
	if maxSendSize > 0 {
		sendData := make([]byte, maxSendSize)
		n, err := socket.snd.buf.Read(sendData)
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

		// Update send buffer sequence number
		socket.snd.NXT += uint32(n)
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
