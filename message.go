package main

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type MsgType int

const (
	KEEPALIVE MsgType = iota // 0
	CHOKE                    // 1
	UNCHOKE
	INTERESTED
	NOTINTERESTED
	HAVE
	BITFIELD
	REQUEST
	CANCEL
	PIECE
)

type Payload struct {
	pieceIndex int
	bitField   []byte
	begin      int
	length     int
	block      []byte
} // last part of the message. contains message content

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

type Message struct {
	Mtype   MsgType
	Length  int
	Payload Payload
}

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
	default:
		return Message{}, errors.New("NewMessage: Unknown message type")
	}

	return msg, nil
}

func getType(msgBytes []byte) MsgType {

	if len(msgBytes) <= 4 {
		return KEEPALIVE
	}

	var msgType int
	binary.Read(bytes.NewReader(msgBytes[4:5]), binary.BigEndian, &msgType)

	return MsgType(msgType)

}
