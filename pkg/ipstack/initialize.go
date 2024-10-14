package ipstack

import (
	"ip-rip-in-peace/pkg/lnxconfig"
)

// InitNode parses a host's lnx file and returns an error for unsuccessful, maybe will add a return for the port for sending RIP updates if it is a router

func CreateIPConfig() lnxconfig.IPConfig {
	// Parse lnx file
	// Return IPConfig
	// May not be necessary given their parser, depends on how extensive it is (I think it makes the config file for you so may not be neccessary)

}

func InitNode(configInfo lnxconfig.IPConfig) (*IPStack, error) {
	// Create interfaces
	// Assign IP addresses
	// Populate forwarding table

	return nil, nil
}

func Init_RIP() {
	// Create routing table
	// Start sending RIP updates/prepare to send them by setting up a timer/connection?
}
