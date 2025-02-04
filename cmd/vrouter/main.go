package main

import (
	"fmt"
	"ip-rip-in-peace/pkg/ipstack"
	"os"
	"ip-rip-in-peace/pkg/lnxconfig"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "--config" {
		fmt.Println("Usage: vrouter --config <lnx file>")
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
	if stack.IPConfig.RoutingMode == lnxconfig.RoutingTypeRIP {
		stack.RegisterHandler(ipstack.RIP_PROTOCOL, ipstack.RIPHandler)   // RIP protocol
	}

	for _, iface := range stack.Interfaces {
		go ipstack.InterfaceListen(iface, stack)
	}

	if stack.IPConfig.RoutingMode == lnxconfig.RoutingTypeRIP {
		// Send initial RIP request
		stack.SendRIPRequest()

		// Start periodic update
	go stack.PeriodicUpdate(stack.IPConfig.RipPeriodicUpdateRate)

		// Start periodic timeout checking
		go stack.RIPTimeoutCheck(stack.IPConfig.RipTimeoutThreshold)
	}

	

	stack.Repl()
}
