package tcpstack

import (
	"time"
)

// TODO: CHANGE DONT BE AN IDIOT AND FORGET

const BUFFER_SIZE uint16 = 65535
// const BUFFER_SIZE uint16 = 3
	
const RTO_MAX_RETRIES = 3

// const MAX_TCP_PAYLOAD = 1400 - int(unsafe.Sizeof(TCPHeader{})) - 20 // 20 is size of IP header
const MAX_TCP_PAYLOAD = 2 

const MIN_RTO = 30 * time.Second

const ZWP_RETRIES = 30
const ZWP_PROBE_INTERVAL = 1 * time.Second

const MSL = 60 * time.Second
