package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	//	"fmt"
	"strings"
)

type PacketHandler interface {
	ReceiverArbitraryPacket(pRead *bufio.Reader) (Message, error)
	SendArbitraryPacket(pWriter *bufio.Writer, packet []byte) error
	ReceiveHandshakePacket(pRead *bufio.Reader, peer Peer, info TorrentInfo) error
	SendHandshakePacket(pWriter *bufio.Writer, info TorrentInfo) error
}

type Packet int

/*
* attemps to read an arbitrary bittorent packet type, waits for data
* @pRead: ptr to bufio.Reader used for readin from TCP socket
* returns: Message struct used to identify message read in, error
* @see SendArbitraryPacket for sending a packet
 */
func (t *Packet) ReceiveArbitraryPacket(pRead *bufio.Reader, num int) (Message, error) {

	// read 1 bytes to find out length
	msgLength := make([]byte, 4)
	msgLength[0], _ = pRead.ReadByte()
	msgLength[1], _ = pRead.ReadByte()
	msgLength[2], _ = pRead.ReadByte()
	msgLength[3], _ = pRead.ReadByte()

	var length int32
	binary.Read(bytes.NewReader(msgLength), binary.BigEndian, &length)

	data, err := readPacket(int(length), pRead)

	if err != nil {
		return Message{}, err
	}
	data = append(msgLength, data...)
	//form message struct
	msg, err := NewMessage(data)

	return msg, err
}

/*
* sends a packet of a given message type
* @pWrite: ptr to bufio.Writer used to write out to TCP socket
* returns: error
* @see: ReadArbitraryPacket for how to read in a packet
 */
func (t *Packet) SendArbitraryPacket(pWrite *bufio.Writer, packet []byte) error {

	//write it out to socket
	return bufferWrite(pWrite, packet)
}

/*
* waits for a handshake message for a given peer, used only at start of connection
* @pRead: ptr to bufio.Reader used for reading from TCP connection
* @peer: Peer struct used to represent the peer the current connection is for
* returns: error
* @see: SendHandshakePacket for how to send a handshake packet
 */
func (t *Packet) ReceiveHandshakePacket(pRead *bufio.Reader, peer Peer, info TorrentInfo) error {
	// read 1 bytes to find out pstrlen
	pstrlen, err := pRead.ReadByte()
	if err != nil {
		return errors.New("Could not read handhake pstr length")
	}
	length := int(pstrlen) + 48 // len += 8 unset bytes + 20 peer id + 20 infohash

	data, err := readPacket(length, pRead)
	data = append([]byte{pstrlen}, data...)
	return parseHandshakePacket(data, peer, info)
}

/*
* writes out a handshake packet to the TCP socket
* @pWrite: ptr to bufio.Writer for TCP connection
* returns: error
* @see: ReceiveHandshakePacket for how to get a handshake packet response
 */
func (t *Packet) SendHandshakePacket(pWrite *bufio.Writer, info TorrentInfo) error {
	hsk := formHandshakePacket(info)
	return bufferWrite(pWrite, hsk)

}

/*
* HELPER
* compute the handshake msg
* returns: byte slice with handshake bytes
 */
func formHandshakePacket(info TorrentInfo) []byte {
	//form the buffer
	buf := new(bytes.Buffer)
	//put the protocol version name in "BitTorrent protocol" usually
	binary.Write(buf, binary.BigEndian, byte(info.ProtoNameLen))
	//its length
	binary.Write(buf, binary.BigEndian, []byte(info.ProtoName))
	//needs 8 'zero' bytes reserved
	var zeros [8]byte
	binary.Write(buf, binary.BigEndian, zeros)
	//put the infoHash in
	binary.Write(buf, binary.BigEndian, []byte(info.InfoHash))
	//put the client id in (our id)
	binary.Write(buf, binary.BigEndian, []byte(info.ClientID))
	return buf.Bytes()
}

/*
* HELPER
* receive a handshake msg, parse its byte, and compare it to what we expect
* returns: error or nil
 */
func parseHandshakePacket(hsk []byte, peer Peer, info TorrentInfo) error {
	//parse and compare the version strlen
	pstrLen := int(hsk[0])
	if pstrLen != info.ProtoNameLen {
		return errors.New("receiveHandshakeMsg: pstrLen doesn't match")
	}
	//parse and compare the version string
	pstr := string(hsk[1 : pstrLen+1])
	if strings.Compare(pstr, info.ProtoName) != 0 {
		return errors.New("receiveHandshakeMsg: pstr doesn't match")
	}
	//parse and compare the info hash
	infoHash := string(hsk[pstrLen+9 : pstrLen+29])
	if strings.Compare(infoHash, info.InfoHash) != 0 {
		return errors.New("receiveHandshakeMsg: infoHasH doesn't match")
	}
	//parse and cmpare the peer id
	peerID := string(hsk[pstrLen+9+20:])
	if strings.Compare(peerID, peer.PeerID) != 0 {
		return errors.New("receiveHandshakeMsg: peerId doesn't match")
	}

	return nil

}

/*
* HELPER
* reads in a packet from the TCP socket
* @length: number of bytes to read in
* @pRead: ptr to bufio.Reader for TCP socket
* returns: bytes from pakcet read in, error
 */
func readPacket(length int, pRead *bufio.Reader) ([]byte, error) {
	data := make([]byte, length, length)
	readData := make([]byte, length, length)
	totalRead := 0
	for totalRead < length {
		nRead, err := pRead.Read(readData)

		if err != nil {
			return nil, errors.New("Could not read packet")
		}
		data = append(data[:totalRead], readData[:nRead]...)
		totalRead += nRead
	}
	return data, nil
}

/*
* HELPER
* verifies that bufio.Writer wrote correctly
* @pWriter: ptr to bufio.Writer for TCP connection to write too
* @data: bytes to write to TCP socket
* returns: error
 */
func bufferWrite(pWriter *bufio.Writer, data []byte) error {

	bytesWritten, err := pWriter.Write(data)

	if err != nil {
		return err
	} else if bytesWritten != len(data) {
		return errors.New("bufferWrite: not all bytes could be written")
	} else if err = pWriter.Flush(); err != nil {
		return errors.New("bufferWrite: could not flush bytes")
	}

	return nil

}
