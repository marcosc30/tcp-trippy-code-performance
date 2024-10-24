# ReadMe

## Design Choices

### Abstractions for the IP Layer and Interfaces

The IP layer and interfaces in this project are built using several key data structures and abstractions, which facilitate the interaction between the `vhost` and `vrouter` programs and the shared IP stack code.

#### Key Data Structures

1. **IPStack**: 
   - The `IPStack` struct is the core abstraction representing the IP layer. It contains:
     - A map of `Interfaces`, each representing a network interface.
     - A `ForwardingTable` for routing decisions.
     - A map of `Handlers` for processing packets based on their protocol.
     - A `Mutex` for thread-safe operations.
     - An `IPConfig` for configuration parameters.

2. **Interface**:
   - Represents a network interface with attributes such as:
     - `Name`, `IPAddr`, and `Netmask` for identification and addressing.
     - `UDPAddr` and `Socket` for network communication.
     - A map of `Neighbors` for tracking adjacent nodes.
     - A `Down` flag to indicate the interface status.

3. **ForwardingTable**:
   - Manages routing entries with methods for adding, looking up, and removing routes.
   - Each entry is a `ForwardingTableEntry` containing:
     - `DestinationPrefix`, `NextHop`, `Interface`, `Metric`, `Source`, and `LastUpdated`.

4. **IPPacket**:
   - Represents an IP packet with fields for source and destination IPs, TTL, protocol, payload, and checksum.
   - Includes methods for marshaling and unmarshaling packets, and calculating checksums.

5. **RIPMessage**:
   - Used for RIP (Routing Information Protocol) messages, with fields for command, number of entries, and entries themselves.

#### Interaction Between `vhost`, `vrouter`, and IP Stack

- **Initialization**:
  - Both `vhost` and `vrouter` programs initialize an `IPStack` instance using the `InitNode` function, which parses configuration files and sets up interfaces, neighbors, and routing tables.

- **Packet Handling**:
  - The `RegisterHandler` method allows both programs to register protocol-specific handlers, such as `PrintPacket` for test packets and `RIPHandler` for RIP packets.

- **Interface Listening**:
  - Both programs use the `InterfaceListen` function to start goroutines that listen for incoming packets on each interface, ensuring continuous packet processing.

- **REPL (Read-Eval-Print Loop)**:
  - The `Repl` method provides an interactive command-line interface for managing the IP stack, allowing users to list interfaces, neighbors, routes, and control interface states.

- **Routing and Forwarding**:
  - The `vrouter` program specifically handles RIP routing, sending periodic updates and processing RIP requests and responses to maintain the routing table.

This architecture allows for modular and flexible handling of network operations, with shared code in the `ipstack` package providing the necessary abstractions and functionality for both `vhost` and `vrouter` programs. If you take a look at `cmd/vhost/main.go` and `cmd/vrouter/main.go`, it is easy to see how similar the programs are, and how the abstractions we made allow for this.

### How you use threads/goroutines
In general, we use threads for various listening/checking tasks for both the routers and host. For hosts, the only go routines used apart from the main thread (that handes REPL for both hosts and routers) is one for each interface to handle listening for incoming packets. 

For routers, we also have a go routine for each interface to listen, but we also have one for sending periodic RIP updates and checking to the forwarding table (which is integrated with the RIP updates) to check for routes that have timed out. 

### The steps you will need to process IP packets

Given an interface receives a packet through UDP (technically on interface listen, we first check to make sure that interface isn't down before continuing to listen), we first unmarshal the packet using a function we made. Then we validate the packet (check the checksum and the TTL) and check if it is for us. If its destination matches one of our interfaces, we send it to the appropriate packet handler based on the packet's protocol (this is done with a function called HandlePacket and with registering protocol handlers in the vhost and vrouter programs). 

If it is not for us, we check if one of the interface netmasks contains the packet destination, in which case the destination is on a directly connected subnet, so we can send the packet directly to the destination IP (we had a function that does this, which itself does it through the interface neighbors). If this is not the case, we apply forwarding logic to it using a function called nextHop (which uses the forwarding table to decide the next hop) and use sendPacket but to send it to the next hop. 

### Other Design Choices


### Found bugs:
- There is a minor bug, since we do the check for an interface being down at the start of each loop in interface listen, since the interface holds while waiting for UDP packets, if the interface is downed while waiting for a UDP packet while in readFromUDP, it will still receive one more packet. I tried fixing this with a read timeout, but this would make it so when the interface was upped, it would suddenly receive a bunch of packets at once. Another possible fix is to add a check in the Recieve function, but this prevents a host from sending a packet to itself when an interface is downed (not sure if this is a bug, but this is different from the given behavior).
