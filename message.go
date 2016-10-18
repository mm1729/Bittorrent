package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// MsgType const enum for Message Type
type MsgType int

const (
	// KEEPALIVE is a message type
	KEEPALIVE MsgType = iota
	// CHOKE is a message type
	CHOKE MsgType = iota
	// UNCHOKE is a message type
	UNCHOKE MsgType = iota
	// INTERESTED is a message type
	INTERESTED MsgType = iota
	// NOTINTERESTED is a message type
	NOTINTERESTED MsgType = iota
	// HAVE is a message type
	HAVE MsgType = iota
	// BITFIELD is a message type
	BITFIELD MsgType = iota
	// REQUEST is a message type
	REQUEST MsgType = iota
	// CANCEL is a message type
	CANCEL MsgType = iota
	// PIECE is a message type
	PIECE
)

// Payload struct containing payload information in a message
type Payload struct {
	pieceIndex int
	bitField   []byte
	begin      int
	length     int
	block      []byte
} // last part of the message. contains message content

// NewPayload creates a payload from byte array
func NewPayload(m MsgType, payloadBytes []byte) Payload {

	var p Payload

	switch m {
	case HAVE:
		// loads index of message into pieceIndex
		binary.Read(bytes.NewReader(payloadBytes), binary.BigEndian, &p.pieceIndex)
	case BITFIELD:
		// whole payloadBytes is the bitfield from the peer
		// which tells you which pieces the peer has
		p.bitField = payloadBytes
	case PIECE: // peer gives you the piece that you requested
		reader := bytes.NewReader(payloadBytes)
		binary.Read(reader, binary.BigEndian, &p.pieceIndex)
		binary.Read(reader, binary.BigEndian, &p.begin)
		binary.Read(reader, binary.BigEndian, &p.block)
	case REQUEST: //requests a piece
		fallthrough
	case CANCEL: //rejects a piece that's just been received
		reader := bytes.NewReader(payloadBytes)
		binary.Read(reader, binary.BigEndian, &p.pieceIndex)
		binary.Read(reader, binary.BigEndian, &p.begin)
		binary.Read(reader, binary.BigEndian, &p.length)
	}

	return p

}

// Message struct containing message type, length and payload stuct
type Message struct {
	Mtype   MsgType
	Length  int
	Payload Payload
}

// NewMessage parses byte array to create a message struct
func NewMessage(msgBytes []byte) (Message, error) {
	var msg Message

	switch msg.Mtype = getType(msgBytes); msg.Mtype {
	case KEEPALIVE:
	case CHOKE:
		fallthrough
	case UNCHOKE:
		fallthrough
	case INTERESTED:
		fallthrough
	case NOTINTERESTED:
		msg.Length = 1 //length of message. Need to not hard code
	case HAVE:
		msg.Length = 5
		msg.Payload = NewPayload(msg.Mtype, msgBytes[5:])
	case REQUEST:
		fallthrough
	case CANCEL:
		msg.Length = 13
		msg.Payload = NewPayload(msg.Mtype, msgBytes[5:])
	case BITFIELD:
		fallthrough
	case PIECE:
		msg.Length = len(msgBytes) - 4
		msg.Payload = NewPayload(msg.Mtype, msgBytes[5:])
	default:
		return Message{}, errors.New("NewMessage: Unknown message type")
	}

	return msg, nil
}

func getType(msgBytes []byte) MsgType {

	if len(msgBytes) <= 4 {
		return KEEPALIVE
	}

	return MsgType(int(msgBytes[4]) + 1)

}

// CreateMessage creates and returns a Message based on the MsgType
func CreateMessage(msgType MsgType, payLoad Payload) (arr []byte, err error) {
	switch msgType {
	case KEEPALIVE:
		arr = []byte{0, 0, 0, 0}
	case CHOKE:
		arr = []byte{0, 0, 0, 1, 0}
	case UNCHOKE:
		arr = []byte{0, 0, 0, 1, 1}
	case INTERESTED:
		arr = []byte{0, 0, 0, 1, 2}
	case NOTINTERESTED:
		arr = []byte{0, 0, 0, 1, 3}
	case HAVE:
		buf := new(bytes.Buffer)
		var length int32 = 5
		var id byte = 4
		binary.Write(buf, binary.BigEndian, length)
		binary.Write(buf, binary.BigEndian, id)
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.pieceIndex))
		arr = buf.Bytes()
	case REQUEST:
		fallthrough
	case CANCEL:
		buf := new(bytes.Buffer)
		var length int32 = 13
		var id byte = 8
		if msgType == REQUEST {
			id = 6
		}
		binary.Write(buf, binary.BigEndian, length)
		binary.Write(buf, binary.BigEndian, id)
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.pieceIndex))
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.begin))
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.length))
		arr = buf.Bytes()
		fmt.Println(arr)
	case PIECE:
		buf := new(bytes.Buffer)
		var length = 9 + int32(len(payLoad.block))
		var id byte = 7
		binary.Write(buf, binary.BigEndian, length)
		binary.Write(buf, binary.BigEndian, id)
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.pieceIndex))
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.begin))
		binary.Write(buf, binary.BigEndian, intToByteArr(payLoad.length))
		arr = buf.Bytes()
	case BITFIELD:
		buf := new(bytes.Buffer)
		var length = 1 + int32(len(payLoad.bitField))
		var id byte = 5
		binary.Write(buf, binary.BigEndian, length)
		binary.Write(buf, binary.BigEndian, id)
		binary.Write(buf, binary.BigEndian, payLoad.bitField)
		arr = buf.Bytes()
	default:
		return nil, errors.New("NewMessage: Unknown message type")
	}
	return arr, nil
}

func intToByteArr(i int) []byte {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(i))
	return bs
}
