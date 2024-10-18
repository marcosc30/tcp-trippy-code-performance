package ipstack

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"log/slog"
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
			fmt.Println("Name / Addr/Prefix / State")
			for _, iface := range s.Interfaces {
				state := "up"
				if iface.Down {
					state = "down"
				}
				fmt.Printf("%s / %s / %s\n", iface.Name, iface.Netmask, state)
				// Check the netmask part, the definition of prefix says length plus one so might be off by one
			}
		case "ln":
			// List neighbors
			// In format Iface / VIP / UDPAddr
			for _, iface := range s.Interfaces {
				if iface.Down {
					continue
				}
				for neighbor, udpaddr := range iface.Neighbors {
					fmt.Printf("%s / %s / %s\n", iface.Name, neighbor, udpaddr)
					// Make sure this is printable
				}
			}
		case "lr":
			// List routes
			fmt.Println("T / Prefix / Next hop / Cost")
			for _, entry := range s.ForwardingTable.Entries {
				fmt.Println("Entry:")
				fmt.Printf("%s", string(entry.Source))
				fmt.Println("")
				fmt.Printf("%s / %s / %s / %d\n", string(entry.Source[0]), entry.DestinationPrefix, entry.NextHop, entry.Metric)
				fmt.Println("donezo")
			}
			fmt.Println("Done")
		case "down":
			// Disable an interface
			// No output expected
			// Command should be formatted as "down <ifname>"
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
			err = s.SendIP(dst, 0, 32, []byte(strings.Join(commands[2:], " ")))
			if err != nil {
				fmt.Println("Error sending packet")
			}
		default:
			fmt.Println("Unknown command")
		}
	}
}

// Passed as function to handle test packets
func PrintPacket(packet *IPPacket, stack *IPStack) {
	slog.Info("Received test packet")
	// Received test packet: Src: <source IP>, Dst: <destination IP>, TTL: <ttl>, Data: <message ...>
	fmt.Printf("Received test packet: Src: %s, Dst: %s, TTL: %d, Data: %s\n", packet.SourceIP, packet.DestinationIP, packet.TTL, string(packet.Payload))
}
