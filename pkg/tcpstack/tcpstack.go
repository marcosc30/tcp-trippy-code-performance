package tcpstack

import (
	"errors"
	"net/netip"
	"sync"
	"ip-rip-in-peace/pkg/ipstack"
)

type TCPStack struct {
	tcpTable []TCPTableEntry
	mutex    sync.Mutex
	ipStack *ipstack.IPStack
	nextPort uint16  // For ephemeral port allocation
}

type TCPTableEntry struct {
	LocalAddress netip.Addr
	LocalPort    uint16
	RemoteAddress netip.Addr
	RemotePort    uint16
	State         TCPState
	SocketStruct  Socket
}

type Socket interface {
	VClose() error
}

type TCPState int

const (
	TCP_LISTEN TCPState = 0
	TCP_SYN_SENT TCPState = 1
	TCP_SYN_RECEIVED TCPState = 2
	TCP_ESTABLISHED TCPState = 3
)

func InitTCPStack(ipStack *ipstack.IPStack) *TCPStack {
	return &TCPStack{
		tcpTable: make([]TCPTableEntry, 0),
		ipStack: ipStack,
		nextPort: 49152, // Start of ephemeral port range
	}
}

func (ts *TCPStack) VInsertTableEntry(entry TCPTableEntry) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.tcpTable = append(ts.tcpTable, entry)
}

func (ts *TCPStack) VDeleteTableEntry(entry TCPTableEntry) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	for i, e := range ts.tcpTable {
		if e == entry {
			ts.tcpTable = append(ts.tcpTable[:i], ts.tcpTable[i+1:]...)
		}
	}
}

// Returns the entry if found, otherwise returns nil and an error
func (ts *TCPStack) VFindTableEntry(localAddress netip.Addr, localPort uint16, remoteAddress netip.Addr, remotePort uint16) (*TCPTableEntry, error) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// First, check if full 4-tuple match
	for i := range ts.tcpTable {
		e := &ts.tcpTable[i]  // Get pointer to avoid copy
		if e.LocalPort == localPort && 
		   e.RemoteAddress == remoteAddress && e.RemotePort == remotePort {
			return e, nil
		}
	}

	// Second, check for listening socket
	for i := range ts.tcpTable {
		e := &ts.tcpTable[i]  // Get pointer to avoid copy
		if e.LocalPort == localPort && 
		   e.State == TCP_LISTEN {
			return e, nil
		}
	}

	return nil, errors.New("entry not found")
}

func (ts *TCPStack) sendPacket(dstAddr netip.Addr, data []byte) error {
    return ts.ipStack.SendIP(dstAddr, ipstack.TCP_PROTOCOL, 16, data)
}

func (ts *TCPStack) allocateEphemeralPort() uint16 {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	
	port := ts.nextPort
	ts.nextPort++
	if ts.nextPort > 65535 {
		ts.nextPort = 49152
	}
	return port
}
