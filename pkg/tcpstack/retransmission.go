package tcpstack

// Here, we will define our logic for retransmission of packets, including RTO calculations

// We need a data structure to track all of the segments sent out, and their associated timers