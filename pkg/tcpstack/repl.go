package tcpstack

import (
	"bufio"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

func (ts *TCPStack) ReplInput(scanner *bufio.Scanner) {
	line := scanner.Text()
	args := strings.Fields(line)

	switch args[0] {
	case "s":
		if len(args) != 3 {
			fmt.Println("Usage: s <socket ID> <bytes>")
			return
		}
		socketID, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid socket ID")

		}
		handleSend(ts, socketID, []byte(args[2]))

	case "r":
		if len(args) != 3 {
			fmt.Println("Usage: r <socket ID> <numbytes>")
			return
		}
		socketID, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid socket ID")
			return
		}
		bytes, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Println("Invalid number of bytes")
			return
		}
		handleRead(ts, socketID, bytes)

	case "a":
		if len(args) != 2 {
			fmt.Println("Usage: a <port>")
			return
		}
		port, err := strconv.ParseUint(args[1], 10, 16)
		if err != nil {
			fmt.Println("Invalid port number")
			return
		}
		go handleAccept(ts, uint16(port))

	case "c":
		if len(args) != 3 {
			fmt.Println("Usage: c <ip> <port>")
			return
		}
		addr, err := netip.ParseAddr(args[1])
		if err != nil {
			fmt.Println("Invalid IP address")
			return
		}
		port, err := strconv.ParseUint(args[2], 10, 16)
		if err != nil {
			fmt.Println("Invalid port number")
			return
		}
		go handleConnect(ts, addr, uint16(port))

	case "ls":
		handleList(ts)

	case "help":
		printHelp()

	case "rtrinfo":
		fmt.Println("Retransmission info:")
		for _, entry := range ts.tcpTable {
			if normalSocket, ok := entry.SocketStruct.(*NormalSocket); ok {
				fmt.Printf("Socket %d: RTO %v, SRTT %v, RTTVAR %v, Receiving Buffer %v\n",
					entry.SocketStruct.GetSID(),
					normalSocket.snd.calculatedRTO,
					normalSocket.snd.SRTT,
					normalSocket.snd.RTTVAR,
					normalSocket.rcv.buf.Length())
			}
		}

	case "rst":
		if len(args) != 2 {
			fmt.Println("Usage: rst <socket ID>")
			return
		}
		socketID, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid socket ID")
			return
		}
		handleRSTSend(ts, socketID)

	case "cl":
		if len(args) != 2 {
			fmt.Println("Usage: cl <socket ID>")
			return
		}
		socketID, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid socket ID")
			return
		}
		handleClose(ts, socketID)

	case "sf":
		if len(args) != 4 {
			fmt.Println("Usage: sf <file path> <addr> <port>")
			return
		}
		filePath := args[1]
		addr, err := netip.ParseAddr(args[2])
		if err != nil {
			fmt.Println("Invalid IP address")
			return
		}
		port, err := strconv.ParseUint(args[3], 10, 16)
		if err != nil {
			fmt.Println("Invalid port number")
			return
		}
		sendFile(ts, filePath, addr, uint16(port))
		
	case "rf":
		if len(args) != 3 {
			fmt.Println("Usage: rf <dest file> <port>")
			return
		}
		destFile := args[1]
		port, err := strconv.ParseUint(args[2], 10, 16)
		if err != nil {
			fmt.Println("Invalid port number")
			return
		}
		receiveFile(ts, destFile, uint16(port))
		
	default:
		fmt.Println("Unknown command. Type 'help' for available commands.")
	}
}

func handleSend(ts *TCPStack, socketID int, data []byte) {
	// ts.mutex.Lock()
	// defer ts.mutex.Unlock()
	socket := ts.getSocketByID(socketID)
	if socket == nil {
		fmt.Println("Invalid socket ID")
		return
	}

	if normalSocket, ok := socket.(*NormalSocket); ok {
		err := normalSocket.VWrite(data)
		if err != nil {
			fmt.Printf("Send error: %v\n", err)
			return
		}

		fmt.Printf("Sent %d bytes\n", len(data))
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func handleRead(ts *TCPStack, socketID int, bytes int) {
	// ts.mutex.Lock()
	// defer ts.mutex.Unlock()

	socket := ts.getSocketByID(socketID)
	if socket == nil {
		fmt.Println("Invalid socket ID")
		return
	}

	if normalSocket, ok := socket.(*NormalSocket); ok {
		data := make([]byte, bytes)
		n, err := normalSocket.VRead(data)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes: %s\n", n, string(data[:n]))
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func handleAccept(ts *TCPStack, port uint16) {
	ls := VListen(ts, port)
	fmt.Printf("Listening on port %d\n", port)

	// Wait for connection
	conn := ls.VAccept()
	if conn != nil {
		// fmt.Printf("Accepted connection from %s:%d\n",
		// 	conn.RemoteAddress, conn.RemotePort)
	}
}

func handleConnect(ts *TCPStack, addr netip.Addr, port uint16) {
	socket := &NormalSocket{}
	err := socket.VConnect(ts, addr, port)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	fmt.Printf("Connected to %s:%d\n", addr, port)
}

func handleList(ts *TCPStack) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	fmt.Println("\nActive TCP connections:")
	fmt.Println("SID\tLocal IP:Port\tRemote IP:Port\tState")
	fmt.Println("-------------------------------------------------------")

	for _, entry := range ts.tcpTable {
		state := getStateString(entry.State)
		fmt.Printf("%d\t%s:%d\t%s:%d\t%s\n",
			entry.SocketStruct.GetSID(),
			entry.LocalAddress, entry.LocalPort,
			entry.RemoteAddress, entry.RemotePort,
			state)
	}
	fmt.Println()
}

func handleClose(ts *TCPStack, socketID int) {
	socket := ts.getSocketByID(socketID)
	if socket == nil {
		fmt.Println("Invalid socket ID")
		return
	}

	if normalSocket, ok := socket.(*NormalSocket); ok {
		err := normalSocket.VClose()
		if err != nil {
			fmt.Printf("Close error: %v\n", err)
			return
		}

		fmt.Println("Closing connection")
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func handleRSTSend(ts *TCPStack, socketID int) {
	socket := ts.getSocketByID(socketID)
	if socket == nil {
		fmt.Println("Invalid socket ID")
		return
	}

	if normalSocket, ok := socket.(*NormalSocket); ok {
		err := normalSocket.sendRST()
		if err != nil {
			fmt.Printf("Send error: %v\n", err)
			return
		}

		fmt.Println("Sent RST")
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func sendFile(ts *TCPStack, filePath string, addr netip.Addr, port uint16) {
	// ts.mutex.Lock()
	// defer ts.mutex.Unlock()

	// See if we are already connected with the destination
	var socketID int
	for _, entry := range ts.tcpTable {
		if entry.RemoteAddress == addr && entry.RemotePort == port {
			socketID = entry.SocketStruct.GetSID()
			break
		}
	}

	// If not, connect to the destination
	if socketID == 0 {
		socket := &NormalSocket{}
		err := socket.VConnect(ts, addr, port)
		if err != nil {
			fmt.Printf("Connection failed: %v\n", err)
			return
		}
		socketID = socket.GetSID()
	}

	// Send the file
	socket := ts.getSocketByID(socketID)
	if normalSocket, ok := socket.(*NormalSocket); ok {
		normalSocket.VSendFile(filePath)
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func receiveFile(ts *TCPStack, destFile string, port uint16) {
	// ts.mutex.Lock()
	// defer ts.mutex.Unlock()

	// See if we are already connected with the source
	var socketID int
	for _, entry := range ts.tcpTable {
		if entry.LocalPort == port {
			socketID = entry.SocketStruct.GetSID()
			break
		}
	}

	// If not, accept the connection
	if socketID == 0 {
		ls := VListen(ts, port)
		fmt.Printf("Listening on port %d\n", port)

		// Wait for connection
		conn := ls.VAccept()
		if conn != nil {
			socketID = conn.GetSID()
		}
	}

	// Receive the file
	socket := ts.getSocketByID(socketID)
	if normalSocket, ok := socket.(*NormalSocket); ok {
		go normalSocket.VReceiveFile(destFile)

		fmt.Printf("Receiving file\n")
	} else {
		fmt.Println("Invalid socket type")
		return
	}
}

func getStateString(state TCPState) string {
	switch state {
	case TCP_LISTEN:
		return "LISTEN"
	case TCP_SYN_SENT:
		return "SYN_SENT"
	case TCP_SYN_RECEIVED:
		return "SYN_RECEIVED"
	case TCP_ESTABLISHED:
		return "ESTABLISHED"
	case TCP_FIN_WAIT_1:
		return "FIN_WAIT_1"
	case TCP_FIN_WAIT_2:
		return "FIN_WAIT_2"
	case TCP_CLOSE_WAIT:
		return "CLOSE_WAIT"
	case TCP_CLOSING:
		return "CLOSING"
	case TCP_TIME_WAIT:
		return "TIME_WAIT"
	case TCP_LAST_ACK:
		return "LAST_ACK"
	case TCP_CLOSED:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  a <port>                         - Accept connections on port")
	fmt.Println("  c <ip> <port>     				- Connect to ip:port")
	fmt.Println("  s <socket> <data> 				- Send data on socket")
	fmt.Println("  r <socket> <bytes> 				- Read bytes from socket")
	fmt.Println("  ls                				- List all TCP connections")
	fmt.Println("  help              				- Show this help message")
	fmt.Println("  rtrinfo           				- Show retransmission info")
	fmt.Println("  cl <socket>       				- Close connection")
	fmt.Println("  sf <file path> <addr> <port> 	- Send file")
	fmt.Println("  rf <dest file> <port> 			- Receive file\n")
	fmt.Println()
}
