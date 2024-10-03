# Milestone
Marcos Carsossa
Taj Mahal

## Description
What are the different parts of your IP stack and what data structures do they use? How do these parts interact (API functions, channels, shared data, etc.)?

API Functions:
- Main Function: Forward Function (takes in forwarding table, IP packet):
    - Will be used by hosts and routers when sending, receiving, or forwarding packets
    ```
    Is packet valid?
        Check sum valid
        TTL > 0
    If not, drop packet
    Check forwarding table, three cases
        My IP, for me!
            Send to os
        Local interface
            Send to interface with send protocol
        Next hop
            Send to next hop with send protocol

    ```
- Forwarding Function (takes in forwarding table, IP packet):
    - Will be used by routers to forward packets
    - Will be used by hosts to send packets
    - Will be used by hosts to receive packets
    - Will be used by routers to receive packets
- Send Function (takes in IP packet, interface):
    ```
    TTL--
    Recompute checksum
    Send to interface
    ```
    

-Interface Thread Function:
    - Will run on each host/router
    - When it receives a packet, it checks its "mac table" which is actually IP addresses and UDP ports, and sends it to the correct port based on the IP in the packet
- Listening Function/main:
    - Every host and router will have a listening function that listens for packets on the UDP port on A thread

API Datastructures:
- Every host and router should have a forwarding table:
    - List (Source: IP address object, Destination: Local interfaces)
- Mac table: (IP address, UDP port)
- IP Packet: (Source IP, Destination IP, TTL, Data)
    - Corresponds to each interface


What fields in the IP packet are read to determine how to forward a packet?
- Source IP
- Destination IP
- TTL
- Payload


What will you do with a packet destined for local delivery (ie, destination IP == your node’s IP)?
We will treat it as any other packet, send it to the interface, and then the interface will send it back on the port to the local host


What happens when a link is disabled? (ie, how is forwarding affected)?
- We should maybe have a timeout, so that if a link hasn't been used in a while, then we send a packet as a "hello" and "hello back" protocol. If there is no response, we have to update the table. If there is nowhere to go, packets will have to be dropped or redirected to another network to another network that may know what to do with it



Initialization:

Host sends a packet to its router (only one option so easy):
- First, you must set up the UDP packet, which can be an IP API function that sets up the IP header using the protocol
- To send it, we can make a function that is the same as the one for forwarding to a specific router
1. Decrements the TTL
2. 

Router receives it: 
This part can be done with a table storing byte values representing source IP addresses (so that bit masks can be applied easily) and the destination IP address (has to keep into account masks so that longest-prefix matching can be implemented)

The forwarding part can be done with a generic IP API function called "forwardPacket" or something, which takes in a table and an IP address and determines destination (we can also run it for hosts, but it would be somewhat redundant since the table is just one source and one destination, as well as the option of it being meant for your IP address)

If the packet is meant for the specific router, it should be dropped, same if the TTL is 0
- If it matches one of the interfaces, send to that interface:
- Also have a generic one to send if you don't know something 
(This part is sus because you might have to implement RIP in a way that finds shortest path, so every router must know a path to every other router)

Forwards it:
- 


If a host sends a packet to itself, does it go to the router then back, or just stay at the host?

Are interfaces just tables in this project or should we treat them as independent threads/hardware, which is more similar to real life?

How do we treat deleted links as mentioned above? How would redirecting it actually work, because we don't want to send it to the previous router/host because that would cause an infinite loop, but we want to be able to search for a new path to destination.