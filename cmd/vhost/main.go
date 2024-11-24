package main

import (
	"bufio"
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"ip-rip-in-peace/pkg/tcpstack"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "--config" {
		fmt.Println("Usage: vhost --config <lnx file>")
		os.Exit(1)
	}

	lnxFileName := os.Args[2]

	ipStack, err := ipstack.InitNode(lnxFileName)
	if err != nil {
		fmt.Println("Error initializing node:", err)
		os.Exit(1)
	}

	tcpStack := tcpstack.InitTCPStack(ipStack)

	// Register TCP handler
	ipStack.RegisterHandler(ipstack.TCP_PROTOCOL, func(packet *ipstack.IPPacket, ipStack *ipstack.IPStack) {
		tcpStack.HandlePacket(packet.SourceIP, packet.DestinationIP, packet.Payload)
	})

	// Start interface listeners
	for _, iface := range ipStack.Interfaces {
		go ipstack.InterfaceListen(iface, ipStack)
	}

	// Start TCP background manager
	go tcpStack.RunBackground()

	combinedREPL(tcpStack, ipStack)
}

func combinedREPL(tcpstack *tcpstack.TCPStack, ipstack *ipstack.IPStack) {
	// Have a separate case for help to explain the different commands for both IP and TCP
	scanner := bufio.NewScanner(os.Stdin)
	// fmt.Println("REPL started. Type 'help' for TCP command instructions, and iphelp for IP command instructions.")

	tcp_args := []string{"a", "c", "ls", "s", "sf", "rf", "r"}
	ip_args := []string{"down", "up", "send", "li", "lr", "ln", "exit"}

	OuterLoop: 
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

			if args[0] == "help" {
				tcpstack.ReplInput(scanner)
				continue OuterLoop
			}

			if args[0] == "iphelp" {
				ipstack.ReplInput(scanner)
				continue OuterLoop
			}

			for _, command := range tcp_args {
				if args[0] == command {
					// Execute TCP command
					tcpstack.ReplInput(scanner)
					continue OuterLoop
				}
			}

			for _, command := range ip_args {
				if args[0] == command {
					// Execute IP command
					ipstack.ReplInput(scanner)
					continue OuterLoop
				}
			}

			fmt.Println("Unknown command. Type 'help' for available commands.")

		}
}
