package tcpstack

import (
	"time"
	// "unsafe"
)

// TODO: CHANGE DONT BE AN IDIOT AND FORGET

// const BUFFER_SIZE uint16 = 65535
const BUFFER_SIZE uint16 = 3
	
const RTO_MAX_RETRIES = 3

// const MAX_TCP_PAYLOAD = 1400 - int(unsafe.Sizeof(TCPHeader{})) - 20
const MAX_TCP_PAYLOAD = 1000

const MIN_RTO = 1 * time.Second 

const ZWP_RETRIES = 30
const ZWP_PROBE_INTERVAL = 1 * time.Second

const MSL = 4 * time.Second

const HANDSHAKE_TIMEOUT = 10 * time.Second