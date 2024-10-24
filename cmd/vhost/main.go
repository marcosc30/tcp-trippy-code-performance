package main

import (
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"os"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "--config" {
		fmt.Println("Usage: vhost --config <lnx file>")
		os.Exit(1)
	}

	lnxFileName := os.Args[2]

	stack, err := ipstack.InitNode(lnxFileName)
	if err != nil {
		fmt.Println("Error initializing node:", err)
		os.Exit(1)
	}

	// Add handler functions
	stack.RegisterHandler(ipstack.TEST_PROTOCOL, ipstack.PrintPacket) // Test protocol

	for _, iface := range stack.Interfaces {
		go ipstack.InterfaceListen(iface, stack)
	}

	stack.Repl()
}