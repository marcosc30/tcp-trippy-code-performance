package ipstack

import (
	"bytes"
	"encoding/binary"
)

// Packet format for RIP
type RIPMessage struct {
	command Command
	num_entries uint16
	entries []RIPMessageEntry
}

type Command uint16

const (
	RIP_REQUEST Command = 1
	RIP_RESPONSE Command = 2
)

type RIPMessageEntry struct {
	cost uint32
	address uint32
	mask uint32
}

// RIP Functions
func MarshalRIPMessage(message RIPMessage) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	err := binary.Write(buf, binary.BigEndian, message.command)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, message.num_entries)
	if err != nil {
		return nil, err
	}

	for _, entry := range message.entries {
		err = binary.Write(buf, binary.BigEndian, entry.cost)
		if err != nil {
			return nil, err
		}

		err = binary.Write(buf, binary.BigEndian, entry.address)
		if err != nil {
			return nil, err
		}

		err = binary.Write(buf, binary.BigEndian, entry.mask)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func UnmarshalRIPMessage(message []byte) (RIPMessage, error) {
	buf := bytes.NewBuffer(message)

	var ripMessage RIPMessage

	err := binary.Read(buf, binary.BigEndian, &ripMessage.command)
	if err != nil {
		return RIPMessage{}, err
	}

	err = binary.Read(buf, binary.BigEndian, &ripMessage.num_entries)
	if err != nil {
		return RIPMessage{}, err
	}

	ripMessage.entries = make([]RIPMessageEntry, ripMessage.num_entries)

	for i := 0; i < int(ripMessage.num_entries); i++ {
		var entry RIPMessageEntry

		err = binary.Read(buf, binary.BigEndian, &entry.cost)
		if err != nil {
			return RIPMessage{}, err
		}

		err = binary.Read(buf, binary.BigEndian, &entry.address)
		if err != nil {
			return RIPMessage{}, err
		}

		err = binary.Read(buf, binary.BigEndian, &entry.mask)
		if err != nil {
			return RIPMessage{}, err
		}

		ripMessage.entries[i] = entry
	}

	return ripMessage, nil
}

