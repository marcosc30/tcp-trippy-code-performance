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

	// stack.InitializeHostDefault()

	// Add handler functions
	stack.RegisterHandler(0, ipstack.PrintPacket) // Test protocol
	// TODO: Add RIP handler

	for _, iface := range stack.Interfaces {
		go ipstack.InterfaceListen(iface, stack)
	}

	stack.Repl()
}