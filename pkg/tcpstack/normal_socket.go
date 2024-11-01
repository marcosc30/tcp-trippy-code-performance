package tcpstack

import (
	"net/netip"
)

type NormalSocket struct {
	LocalAddress netip.Addr
	LocalPort uint16
	RemoteAddress netip.Addr
	RemotePort uint16
	SeqNum uint32
	AckNum uint32
	tcpStack *TCPStack
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

	// Send SYN packet
	header := &TCPHeader{
		SourcePort: ns.LocalPort,
		DestPort:   remotePort,
		SeqNum:     ns.SeqNum,
		DataOffset: 5, // 5 32-bit words = 20 bytes
		Flags:      TCP_SYN,
		WindowSize: 65535,
	}

	// Serialize and send packet
	packet := serializeTCPPacket(header, nil)
	err := tcpStack.sendPacket(remoteAddress, packet)
	if err != nil {
		return err
	}

	// TODO: Wait for connection to be established or timeout
	return nil
}

func (ns *NormalSocket) VWrite(tcpStack *TCPStack, data []byte) (int, error) {
	// TODO: Implement data sending
	return 0, nil
}

func (ns *NormalSocket) VRead(tcpStack *TCPStack, data []byte) error {
	// TODO: Implement data receiving
	return nil
}
