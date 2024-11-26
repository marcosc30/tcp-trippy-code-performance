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
	zeroWindowProbing bool
	probeInterval    time.Duration
	probeTimer       *time.Timer
}

const TCP_RETRIES = 3
const ZWP_PROBE_INTERVAL = 1 * time.Second

func (ns *NormalSocket) GetSID() int {
	return ns.SID
}

func (ns *NormalSocket) VClose() error {
	// Check if connection is in the right state
	table_entry, _ := ns.tcpStack.VFindTableEntry(ns.LocalAddress, ns.LocalPort, ns.RemoteAddress, ns.RemotePort)
	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_CLOSE_WAIT {
		return fmt.Errorf("connection not established")
	}

	// Send FIN packet
	header := &TCPHeader{
		SourcePort: ns.LocalPort,
		DestPort:   ns.RemotePort,
		SeqNum:     ns.SeqNum,
		AckNum:     ns.rcv.NXT,
		DataOffset: 5,
		Flags:      TCP_FIN,
		WindowSize: ns.rcv.WND,
	}

	ns.snd.inFlightPackets.mutex.Lock()
	ns.snd.inFlightPackets.packets = append(ns.snd.inFlightPackets.packets, InFlightPacket{
		data:     nil,
		SeqNum:   ns.SeqNum,
		Length:   0,
		timeSent: time.Now(),
		flags:    TCP_FIN,
	})
	ns.snd.inFlightPackets.mutex.Unlock()

	ns.snd.RTOtimer.Reset(ns.snd.calculatedRTO)

	packet := serializeTCPPacket(header, nil)
	err := ns.tcpStack.sendPacket(ns.RemoteAddress, packet)
	if err != nil {
		return err
	}

	// Update state to FIN_WAIT_1
	table_entry, _ = ns.tcpStack.VFindTableEntry(ns.LocalAddress, ns.LocalPort, ns.RemoteAddress, ns.RemotePort)
	if table_entry.State == TCP_ESTABLISHED {
		table_entry.State = TCP_FIN_WAIT_1
	} else if table_entry.State == TCP_CLOSE_WAIT {
		table_entry.State = TCP_LAST_ACK
	}

	return nil
}

func (ns *NormalSocket) VConnect(tcpStack *TCPStack, remoteAddress netip.Addr, remotePort uint16) error {
	ns.SID = tcpStack.generateSID()
	ns.tcpStack = tcpStack
	ns.RemoteAddress = remoteAddress
	ns.RemotePort = remotePort
	ns.LocalPort = tcpStack.allocatePort()
	ns.SeqNum = generateInitialSeqNum()
	ns.lastActive = time.Now()

	// Initialize send/receive state
	ns.snd = SND{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		ISS: ns.SeqNum,
		UNA: ns.SeqNum,
		NXT: ns.SeqNum + 1, // +1 for SYN
		WND: BUFFER_SIZE,
		//RTOtimer:      time.NewTimer(1 * time.Second), // This is the default value
		calculatedRTO:   1 * time.Second,
		SRTT:            0,
		RTTVAR:          0,
		retransmissions: 0,
	}
	ns.snd.buf.SetBlocking(true)
	ns.snd.RTOtimer = time.NewTimer(ns.snd.calculatedRTO)
	ns.snd.RTOtimer.Stop()

	ns.rcv = RCV{
		buf: ringbuffer.New(int(BUFFER_SIZE)),
		WND: BUFFER_SIZE,
	}
	ns.rcv.buf.SetBlocking(true)

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

	// Check if connection is in the right state

	table_entry, err := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
	if err != nil {
		fmt.Println("Error finding table entry: ", err)
		return err
	}

	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_CLOSE_WAIT {
		return fmt.Errorf("connection not established")
	}

	// Write data to send buffer
	_, err = socket.snd.buf.Write(data)
	if err != nil {
		return err
	}

	// What is the behavior if there is more data than the send buffer can handle

	// Start the RTO timer
	socket.snd.RTOtimer.Reset(socket.snd.calculatedRTO)

	// We start the RTO timer here so that it is per write, not per packet

	// Try to send immediately
	return socket.trySendData()
}

func (socket *NormalSocket) trySendData() error {
	if socket.snd.WND == 0 {
		if !socket.zeroWindowProbing {
			socket.zeroWindowProbing = true
			socket.probeInterval = ZWP_PROBE_INTERVAL
			socket.startProbing()
		}
		return nil
	}
	
	// Reset probing state if window opened
	if socket.zeroWindowProbing {
		socket.zeroWindowProbing = false
		socket.probeTimer.Stop()
	}
	
	// ... rest of existing send logic ...
}

func (socket *NormalSocket) startProbing() {
	socket.probeTimer = time.NewTimer(socket.probeInterval)
	go func() {
		for socket.zeroWindowProbing {
			<-socket.probeTimer.C
			
			// Send 1-byte probe
			header := &TCPHeader{
				SourcePort: socket.LocalPort,
				DestPort:   socket.RemotePort,
				SeqNum:     socket.snd.NXT,
				AckNum:     socket.rcv.NXT,
				DataOffset: 5,
				Flags:      TCP_ACK,
				WindowSize: socket.rcv.WND,
			}
			
			socket.tcpStack.sendPacket(socket.RemoteAddress, 
				serializeTCPPacket(header, []byte{0}))
			
			// Exponential backoff
			socket.probeInterval = min(socket.probeInterval * 2, 
				60 * time.Second)
			socket.probeTimer.Reset(socket.probeInterval)
		}
	}()
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
	// Check if connection is in the right state
	table_entry, _ := socket.tcpStack.VFindTableEntry(socket.LocalAddress, socket.LocalPort, socket.RemoteAddress, socket.RemotePort)
	if table_entry.State != TCP_ESTABLISHED && table_entry.State != TCP_FIN_WAIT_1 && table_entry.State != TCP_FIN_WAIT_2 {
		return 0, fmt.Errorf("connection not established")
	}

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
