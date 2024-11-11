package tcpstack

import (
	"bufio"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// func (ts *TCPStack) Repl() {
// 	scanner := bufio.NewScanner(os.Stdin)
// 	fmt.Println("TCP REPL started. Type 'help' for commands.")

// 	for {
// 		fmt.Print("> ")
// 		if !scanner.Scan() {
// 			break
// 		}

// 		line := scanner.Text()
// 		args := strings.Fields(line)
// 		if len(args) == 0 {
// 			continue
// 		}

// 		switch args[0] {
// 		case "s":
// 			if len(args) != 3 {
// 				fmt.Println("Usage: s <socket ID> <bytes>")
// 				continue
// 			}
// 			socketID, err := strconv.Atoi(args[1])
// 			if err != nil {
// 				fmt.Println("Invalid socket ID")
// 				continue
// 			}
// 			handleSend(ts, socketID, []byte(args[2]))

// 		case "r":
// 			if len(args) != 3 {
// 				fmt.Println("Usage: r <socket ID> <numbytes>")
// 				continue
// 			}
// 			socketID, err := strconv.Atoi(args[1])
// 			if err != nil {
// 				fmt.Println("Invalid socket ID")
// 				continue
// 			}
// 			bytes, err := strconv.Atoi(args[2])
// 			if err != nil {
// 				fmt.Println("Invalid number of bytes")
// 				continue
// 			}
// 			handleRead(ts, socketID, bytes)

// 		case "a":
// 			if len(args) != 2 {
// 				fmt.Println("Usage: a <port>")
// 				continue
// 			}
// 			port, err := strconv.ParseUint(args[1], 10, 16)
// 			if err != nil {
// 				fmt.Println("Invalid port number")
// 				continue
// 			}
// 			go handleAccept(ts, uint16(port))

// 		case "c":
// 			if len(args) != 3 {
// 				fmt.Println("Usage: c <ip> <port>")
// 				continue
// 			}
// 			addr, err := netip.ParseAddr(args[1])
// 			if err != nil {
// 				fmt.Println("Invalid IP address")
// 				continue
// 			}
// 			port, err := strconv.ParseUint(args[2], 10, 16)
// 			if err != nil {
// 				fmt.Println("Invalid port number")
// 				continue
// 			}
// 			go handleConnect(ts, addr, uint16(port))

// 		case "ls":
// 			handleList(ts)

// 		case "help":
// 			printHelp()

// 		default:
// 			fmt.Println("Unknown command. Type 'help' for available commands.")
// 		}
// 	}
// }

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

	default:
		fmt.Println("Unknown command. Type 'help' for available commands.")
	}
}

func handleSend(ts *TCPStack, socketID int, data []byte) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if socketID < 0 || socketID >= len(ts.tcpTable) {
		fmt.Println("Invalid socket ID")
		return
	}

	socket := ts.getSocketByID(socketID)
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
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if socketID < 0 || socketID >= len(ts.tcpTable) {
		fmt.Println("Invalid socket ID")
		return
	}

	socket := ts.getSocketByID(socketID)
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
	default:
		return "UNKNOWN"
	}
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  a <port>          - Accept connections on port")
	fmt.Println("  c <ip> <port>     - Connect to ip:port")
	fmt.Println("  s <socket> <data> - Send data on socket")
	fmt.Println("  r <socket> <bytes> - Read bytes from socket")
	fmt.Println("  ls                - List all TCP connections")
	fmt.Println("  help              - Show this help message")
	fmt.Println()
}
