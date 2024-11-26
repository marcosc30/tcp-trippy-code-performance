package tcpstack

import (
	"errors"
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"net/netip"
	"sync"
	"time"
	"github.com/smallnest/ringbuffer"
)

const BUFFER_SIZE uint16 = 65535
// const BUFFER_SIZE uint16 = 3

type TCPStack struct {
	tcpTable []TCPTableEntry
	mutex    sync.Mutex
	ipStack  *ipstack.IPStack
	// rcv      RCV
	// snd      SND
	// snd and rcv should be on the level of socket connection, not the stack which is a per host/client level
	nextPort uint16 // For ephemeral port allocation
	nextSID  int
}

type SND struct {
	buf           *ringbuffer.RingBuffer
	UNA           uint32        // oldest unacknowledged sequence number
	NXT           uint32        // next sequence number to be sent
	WND           uint16        // peer's advertised window size
	ISS           uint32        // initial send sequence number
	calculatedRTO time.Duration // RTO for that connection, calculated based on RTT
	RTOtimer      *time.Timer   // Timer for RTO
	SRTT          time.Duration // Smoothed RTT
	RTTVAR        time.Duration // RTT variance
	retransmissions int

	// add the retransmission/in flight packet tracker, which could be a stack containing all of the segments (with each segment being data, the sequence number, length of segment, and the time it was last sent)
	inFlightPackets InFlightPacketStack
}

type InFlightPacket struct {
	data            []byte // This may be too much overhead to track the data of every in flight packet
	SeqNum          uint32
	Length          uint16
	timeSent        time.Time
	//CalculatedRTO time.Duration // This should be done per connection, not per packet
}

type InFlightPacketStack struct {
	packets []InFlightPacket
	mutex  sync.Mutex
}

type RCV struct {
	buf *ringbuffer.RingBuffer
	WND uint16
	NXT uint32 // next expected sequence number
	IRS uint32 // initial receive sequence number

	earlyData []EarlyData
}

type EarlyData struct {
	data []byte
	SeqNum uint32
	Length uint16
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
	TCP_FIN_WAIT_1   TCPState = 4
	TCP_FIN_WAIT_2   TCPState = 5
	TCP_CLOSING      TCPState = 6
	TCP_TIME_WAIT    TCPState = 7
	TCP_CLOSE_WAIT   TCPState = 8
	TCP_LAST_ACK     TCPState = 9
	TCP_CLOSED       TCPState = 10
)

func InitTCPStack(ipStack *ipstack.IPStack) *TCPStack {
	fmt.Println("BUFFER_SIZE: ", BUFFER_SIZE)

	result := &TCPStack{
		tcpTable: make([]TCPTableEntry, 0),
		ipStack:  ipStack,
		nextPort: 49152, // Start of ephemeral port range
		// rcv: RCV{
		// 	buf: ringbuffer.New(int(BUFFER_SIZE)),
		// 	NXT: 0,
		// 	IRS: 0,
		// },
		// snd: SND{
		// 	buf: ringbuffer.New(int(BUFFER_SIZE)),
		// 	UNA: 0,
		// 	NXT: 0,
		// 	WND: 0,
		// 	ISS: 0,
		// },
		nextSID: 0,
	}

	// This is not blocking, it is erroring on a read, we need to call it before any read or write calls
	// Sending when buffer is full should also block
	// result.rcv.buf.SetBlocking(true)
	// result.snd.buf.SetBlocking(true)
	// No wonder this wasn't blocking, it was never used

	return result
}

func (ts *TCPStack) generateSID() int {
	nextSID := ts.nextSID
	ts.nextSID++
	return nextSID
}

func (ts *TCPStack) getSocketByID(id int) Socket {
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
	switch entry.SocketStruct.(type) {
	case *NormalSocket:
		go entry.SocketStruct.(*NormalSocket).manageRetransmissions()
	}
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
		e := &ts.tcpTable[i]
		if e.LocalPort == localPort &&
			e.RemoteAddress == remoteAddress && e.RemotePort == remotePort {
			return e, nil
		}
	}

	// Second, check for listening socket
	for i := range ts.tcpTable {
		e := &ts.tcpTable[i]
		if e.LocalPort == localPort &&
			e.State == TCP_LISTEN {
			return e, nil
		}
	}

	fmt.Println("entry not found")
	return nil, errors.New("entry not found")
}

func (ts *TCPStack) sendPacket(dstAddr netip.Addr, data []byte) error {
	return ts.ipStack.SendIP(dstAddr, ipstack.TCP_PROTOCOL, 16, data)
}

func (ts *TCPStack) allocatePort() uint16 {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	port := ts.nextPort
	ts.nextPort++
	if ts.nextPort == 0 { //uint wraps
		ts.nextPort = 49152
	}
	return port
}

