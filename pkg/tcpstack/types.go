package tcpstack

import (
	"net/netip"
	"sync"
	"time"
	"github.com/smallnest/ringbuffer"
	"ip-rip-in-peace/pkg/ipstack"
)

type TCPHeader struct {
	SourcePort uint16
	DestPort   uint16
	SeqNum     uint32
	AckNum     uint32
	DataOffset uint8 // 4 bits
	Flags      uint8 // 8 bits
	WindowSize uint16
	Checksum   uint16
	UrgentPtr  uint16
}

const (
	TCP_FIN = 1 << 0
	TCP_SYN = 1 << 1
	TCP_RST = 1 << 2
	TCP_PSH = 1 << 3
	TCP_ACK = 1 << 4
)


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
	buf             *ringbuffer.RingBuffer
	UNA             uint32        // oldest unacknowledged sequence number
	NXT             uint32        // next sequence number to be sent
	WND             uint16        // peer's advertised window size
	ISS             uint32        // initial send sequence number
	calculatedRTO   time.Duration // RTO for that connection, calculated based on RTT
	RTOtimer        *time.Timer   // Timer for RTO
	SRTT            time.Duration // Smoothed RTT
	RTTVAR          time.Duration // RTT variance
	retransmissions int

	// add the retransmission/in flight packet tracker, which could be a stack containing all of the segments (with each segment being data, the sequence number, length of segment, and the time it was last sent)
	inFlightPackets InFlightPacketStack
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
	establishedChan chan struct{}
}


type ListenSocket struct {
	SID int
	localPort uint16
	// Channel for pending connections
	acceptQueue chan *NormalSocket
}