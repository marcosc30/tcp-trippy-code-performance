package tcpstack

import (
	"errors"
	"ip-rip-in-peace/pkg/ipstack"
	"net/netip"
	"os"
	"sync"

	"github.com/smallnest/ringbuffer"
)

const BUFFER_SIZE uint16 = 65535

type TCPStack struct {
	tcpTable []TCPTableEntry
	mutex    sync.Mutex
	ipStack  *ipstack.IPStack
	rcv      RCV
	snd      SND
	nextPort uint16 // For ephemeral port allocation
	nextSID  int
}

type SND struct {
	buf *ringbuffer.RingBuffer
	UNA uint32
	NXT uint32
	WND uint16
	ISS uint32

	writeReady chan struct{}
}

type RCV struct {
	buf *ringbuffer.RingBuffer
	NXT uint32
	WND uint16
	IRS uint32

	dataReady chan struct{}
}

type TCPTableEntry struct {
	LocalAddress  netip.Addr
	LocalPort     uint16
	RemoteAddress netip.Addr
	RemotePort    uint16
	State         TCPState
	SocketStruct  Socket
}

type Socket interface {
	VClose() error
	GetSID() int
}

type TCPState int

const (
	TCP_LISTEN       TCPState = 0
	TCP_SYN_SENT     TCPState = 1
	TCP_SYN_RECEIVED TCPState = 2
	TCP_ESTABLISHED  TCPState = 3
)

func InitTCPStack(ipStack *ipstack.IPStack) *TCPStack {
	result := &TCPStack{
		tcpTable: make([]TCPTableEntry, 0),
		ipStack:  ipStack,
		nextPort: 49152, // Start of ephemeral port range
		rcv: RCV{
			buf: ringbuffer.New(int(BUFFER_SIZE)),
			NXT: 0,
			WND: 0,
			IRS: 0,
		},
		snd: SND{
			buf: ringbuffer.New(int(BUFFER_SIZE)),
			UNA: 0,
			NXT: 0,
			WND: 0,
			ISS: 0,
		},
		nextSID: 0,
	}

	result.rcv.buf.SetBlocking(true)
	result.snd.buf.SetBlocking(true)

	return result
}

func (ts *TCPStack) generateSID() int {
	nextSID := ts.nextSID
	ts.nextSID++
	return nextSID
}

func (ts *TCPStack) getSocketByID(id int) Socket {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	for _, e := range ts.tcpTable {
		if e.SocketStruct.GetSID() == id {
			return e.SocketStruct
		}
	}
	return nil
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
		e := &ts.tcpTable[i] // Get pointer to avoid copy
		if e.LocalPort == localPort &&
			e.RemoteAddress == remoteAddress && e.RemotePort == remotePort {
			return e, nil
		}
	}

	// Second, check for listening socket
	for i := range ts.tcpTable {
		e := &ts.tcpTable[i] // Get pointer to avoid copy
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
	if ts.nextPort == 0 { //uint wraps
		ts.nextPort = 49152
	}
	return port
}

func (ts *TCPStack) sendFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func (ts *TCPStack) receiveFile() error {
	return nil
}
