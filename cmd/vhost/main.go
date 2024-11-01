package main

import (
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"ip-rip-in-peace/pkg/tcpstack"
	"os"
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

	// Use TCP REPL instead of IP REPL
	tcpStack.Repl()
}