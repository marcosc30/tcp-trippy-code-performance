package tcpstack

import (
	"errors"
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"net/netip"
	"encoding/binary"
)

func InitTCPStack(ipStack *ipstack.IPStack) *TCPStack {
	fmt.Println("BUFFER_SIZE: ", BUFFER_SIZE)

	result := &TCPStack{
		tcpTable: make([]TCPTableEntry, 0),
		ipStack:  ipStack,
		nextPort: 49152, // Start of ephemeral port range
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
	// Get source IP from the first interface
	var srcIP []byte
	for _, iface := range ts.ipStack.Interfaces {
		srcIP = iface.IPAddr.AsSlice()
		break
	}
	
	// Calculate TCP checksum with pseudo header
	checksum := computeChecksum(
		srcIP,
		dstAddr.AsSlice(),
		uint8(ipstack.TCP_PROTOCOL),
		data,
	)
	
	// Insert TCP checksum into packet
	binary.BigEndian.PutUint16(data[16:18], checksum)
	
	// fmt.Printf("Sending TCP packet:\n")
	// fmt.Printf("  Length: %d\n", len(data))
	// fmt.Printf("  Source IP: %v\n", srcIP)
	// fmt.Printf("  Dest IP: %v\n", dstAddr.AsSlice())
	// fmt.Printf("  First 20 bytes: %v\n", data[:20])
	
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
