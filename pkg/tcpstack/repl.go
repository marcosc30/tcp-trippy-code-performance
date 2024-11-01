package tcpstack

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

func (ts *TCPStack) Repl() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("TCP REPL started. Type 'help' for commands.")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		switch args[0] {
		case "a":
			if len(args) != 2 {
				fmt.Println("Usage: a <port>")
				continue
			}
			port, err := strconv.ParseUint(args[1], 10, 16)
			if err != nil {
				fmt.Println("Invalid port number")
				continue
			}
			go handleAccept(ts, uint16(port))

		case "c":
			if len(args) != 3 {
				fmt.Println("Usage: c <ip> <port>")
				continue
			}
			addr, err := netip.ParseAddr(args[1])
			if err != nil {
				fmt.Println("Invalid IP address")
				continue
			}
			port, err := strconv.ParseUint(args[2], 10, 16)
			if err != nil {
				fmt.Println("Invalid port number")
				continue
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
}

func handleAccept(ts *TCPStack, port uint16) {
	ls := VListen(ts, port)
	fmt.Printf("Listening on port %d\n", port)

	// Wait for connection
	conn := ls.VAccept()
	if conn != nil {
		fmt.Printf("Accepted connection from %s:%d\n", 
			conn.RemoteAddress, conn.RemotePort)
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
	fmt.Println("Local Address:Port\tRemote Address:Port\tState")
	fmt.Println("-------------------------------------------------------")
	
	for _, entry := range ts.tcpTable {
		state := getStateString(entry.State)
		fmt.Printf("%s:%d\t%s:%d\t%s\n",
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
	fmt.Println("  ls                - List all TCP connections")
	fmt.Println("  help              - Show this help message")
	fmt.Println()
} 