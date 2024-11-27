package tcpstack

import (
	"net/netip"
)

func VListen(tcpStack *TCPStack, localPort uint16) *ListenSocket {
	ls := &ListenSocket{
		SID: tcpStack.generateSID(),
		localPort: localPort,
		acceptQueue: make(chan *NormalSocket, 10), // Buffer size of 10
	}

	tcpStack.VInsertTableEntry(TCPTableEntry{
		LocalAddress: netip.AddrFrom4([4]byte{0, 0, 0, 0}),
		LocalPort: localPort,
		RemoteAddress: netip.AddrFrom4([4]byte{0, 0, 0, 0}),
		RemotePort: 0,
		State: TCP_LISTEN,
		SocketStruct: ls,
	})

	return ls
}

func (ls *ListenSocket) GetSID() int {
	return ls.SID
}

func (ls *ListenSocket) VAccept() *NormalSocket {
	// Block until a connection is received on the accept queue
	return <-ls.acceptQueue
}

func (ls *ListenSocket) VClose() error {
	return nil
}
