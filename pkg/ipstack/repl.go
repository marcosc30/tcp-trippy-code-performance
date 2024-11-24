package ipstack

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

// Reference file:
// https://brown-csci1680.github.io/iptcp-docs/specs/repl-commands/

func (s *IPStack) Repl() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()

		commands := strings.Split(line, " ")

		switch commands[0] {
		case "li":
			// List interfaces
			// In format Name / Addr/Prefix / State
			fmt.Println("Name Addr/Prefix State")
			for _, iface := range s.Interfaces {
				state := "up"
				if iface.Down {
					state = "down"
				}
				fmt.Printf("%s %s %s\n", iface.Name, iface.IPAddr, state)
				// Check the netmask part, the definition of prefix says length plus one so might be off by one
			}
		case "ln":
			// List neighbors
			// In format Iface / VIP / UDPAddr
			fmt.Println("Iface VIP UDPAddr")
			for _, iface := range s.Interfaces {
				if iface.Down {
					continue
				}
				for neighbor, udpaddr := range iface.Neighbors {
					fmt.Printf("%s %s %s\n", iface.Name, neighbor, udpaddr)
					// Make sure this is printable
				}
			}
		case "lr":
			// List routes
			fmt.Println("T Prefix Next hop Cost")
			for _, entry := range s.ForwardingTable.Entries {
				// For local routes, print LOCAL:<ifname>
				if entry.Source == SourceLocal {
					fmt.Printf("L %s LOCAL:%s 0\n", entry.DestinationPrefix, entry.Interface)
					continue
				}
				// For static routes, print with cost "-"
				if entry.Source == SourceStatic {
					fmt.Printf("S %s %s -\n", entry.DestinationPrefix, entry.NextHop)
					continue
				}
				fmt.Printf("%s %s %s %d\n", string(entry.Source[0]), entry.DestinationPrefix, entry.NextHop, entry.Metric)
			}
		case "down":
			// Disable an interface
			// No output expected
			// Command should be formatted as "down <ifname>"
			if len(commands) != 2 {
				fmt.Println("Usage: down <ifname>")
				continue
			}
			ifname := commands[1]
			for _, iface := range s.Interfaces {
				if iface.Name == ifname {
					iface.Down = true
					break
				}
			}
		case "up":
			// Enable an interface
			// No output expected
			// Command should be formatted as "up <ifname>"
			if len(commands) != 2 {
				fmt.Println("Usage: up <ifname>")
				continue
			}
			ifname := commands[1]
			for _, iface := range s.Interfaces {
				if iface.Name == ifname {
					iface.Down = false
					break
				}
			}
		case "send":
			// Send a test packet
			// No output expected
			// Command should be formatted as <addr> <message ...>
			dst, err := netip.ParseAddr(commands[1])
			if err != nil {
				fmt.Println("Error parsing address")
				continue
			}
			if len(commands) < 3 {
				fmt.Println("Usage: send <addr> <message ...>")
				continue
			}
			err = s.SendIP(dst, TEST_PROTOCOL, 16 + 1, []byte(strings.Join(commands[2:], " ")))
			if err != nil {
				fmt.Println("Error sending packet")
			}
		case "exit":
			// Quit process
			os.Exit(0)
		default:
			fmt.Println("Unknown command")
		}
	}
}

func (s *IPStack) ReplInput(scanner *bufio.Scanner) {
	line := scanner.Text()

	commands := strings.Fields(line)

	switch commands[0] {
	case "li":
		// List interfaces
		// In format Name / Addr/Prefix / State
		fmt.Println("Name Addr/Prefix State")
		for _, iface := range s.Interfaces {
			state := "up"
			if iface.Down {
				state = "down"
			}
			fmt.Printf("%s %s %s\n", iface.Name, iface.IPAddr, state)
			// Check the netmask part, the definition of prefix says length plus one so might be off by one
		}
	case "ln":
		// List neighbors
		// In format Iface / VIP / UDPAddr
		fmt.Println("Iface VIP UDPAddr")
		for _, iface := range s.Interfaces {
			if iface.Down {
				continue
			}
			for neighbor, udpaddr := range iface.Neighbors {
				fmt.Printf("%s %s %s\n", iface.Name, neighbor, udpaddr)
				// Make sure this is printable
			}
		}
	case "lr":
		// List routes
		fmt.Println("T Prefix Next hop Cost")
		for _, entry := range s.ForwardingTable.Entries {
			// For local routes, print LOCAL:<ifname>
			if entry.Source == SourceLocal {
				fmt.Printf("L %s LOCAL:%s 0\n", entry.DestinationPrefix, entry.Interface)
				continue
			}
			// For static routes, print with cost "-"
			if entry.Source == SourceStatic {
				fmt.Printf("S %s %s -\n", entry.DestinationPrefix, entry.NextHop)
				continue
			}
			fmt.Printf("%s %s %s %d\n", string(entry.Source[0]), entry.DestinationPrefix, entry.NextHop, entry.Metric)
		}
	case "down":
		// Disable an interface
		// No output expected
		// Command should be formatted as "down <ifname>"
		if len(commands) != 2 {
			fmt.Println("Usage: down <ifname>")
			return
		}
		ifname := commands[1]
		for _, iface := range s.Interfaces {
			if iface.Name == ifname {
				iface.Down = true
				break
			}
		}
	case "up":
		// Enable an interface
		// No output expected
		// Command should be formatted as "up <ifname>"
		if len(commands) != 2 {
			fmt.Println("Usage: up <ifname>")
			return
		}
		ifname := commands[1]
		for _, iface := range s.Interfaces {
			if iface.Name == ifname {
				iface.Down = false
				break
			}
		}
	case "send":
		// Send a test packet
		// No output expected
		// Command should be formatted as <addr> <message ...>
		dst, err := netip.ParseAddr(commands[1])
		if err != nil {
			fmt.Println("Error parsing address")
			return
		}

		if len(commands) < 3 {
			fmt.Println("Usage: send <addr> <message ...>")
			return	
		}

		err = s.SendIP(dst, TEST_PROTOCOL, 16 + 1, []byte(strings.Join(commands[2:], " ")))
		if err != nil {
			fmt.Println("Error sending packet")
		}
	case "exit":
		// Quit process
		os.Exit(0)
	case "iphelp":
		fmt.Println("IP commands:")
		fmt.Println("li: List interfaces")
		fmt.Println("ln: List neighbors")
		fmt.Println("lr: List routes")
		fmt.Println("down <ifname>: Disable an interface")
		fmt.Println("up <ifname>: Enable an interface")
		fmt.Println("send <addr> <message ...>: Send a test packet")
		fmt.Println("exit: Quit process")
	default:
		fmt.Println("Unknown command")
	}
}

// Passed as function to handle test packets
func 	PrintPacket(packet *IPPacket, stack *IPStack) {
	// Received test packet: Src: <source IP>, Dst: <destination IP>, TTL: <ttl>, Data: <message ...>
	fmt.Printf("Received test packet: Src: %s, Dst: %s, TTL: %d, Data: %s\n", packet.SourceIP, packet.DestinationIP, packet.TTL, string(packet.Payload))
}
