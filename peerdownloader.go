package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

//TorrentInfo used to consolidate space, not the same as InfoDict
type TorrentInfo struct {
	TInfo        *InfoDict //torrent info dict
	ClientID     string    //peer id client chose
	ProtoName    string    //bittorent protocol version
	ProtoNameLen int       //length
	InfoHash     string    //hash for this torrent
}

//PeerDownloader used to communicate with the list of peers
type PeerDownloader struct {
	info     TorrentInfo  //information about the torrent [see above]
	peerList []Peer       //list of RU peer
	manager  PieceManager //manages requests for pieces
}

/*
NewPeerDownloader create a new peerdownloader
* @tInfo: torrent info dictionary
* @peers: slice of Peer with RU
* @clientId: user selected peer-id
* @protoName: protocol version name for this version of BT
* returns: new PeerDownloader
*/
func NewPeerDownloader(tInfo TorrentInfo, peers []Peer) PeerDownloader {
	var p PeerDownloader

	p.info = tInfo
	p.peerList = peers
	p.manager = NewPieceManager(tInfo.TInfo, 10)

	return p
}

/*
* compute the handshake msg
* returns: byte slice with handshake bytes
 */
func (t *PeerDownloader) getHandshakeMsg() []byte {
	//form the buffer
	buf := new(bytes.Buffer)
	//put the protocol version name in "BitTorrent protocol" usually
	binary.Write(buf, binary.BigEndian, byte(t.info.ProtoNameLen))
	//its length
	binary.Write(buf, binary.BigEndian, []byte(t.info.ProtoName))
	//needs 8 'zero' bytes reserved
	var zeros [8]byte
	binary.Write(buf, binary.BigEndian, zeros)
	//put the infoHash in
	binary.Write(buf, binary.BigEndian, []byte(t.info.InfoHash))
	//put the client id in (our id)
	binary.Write(buf, binary.BigEndian, []byte(t.info.ClientID))
	return buf.Bytes()
}

/*
* receive a handshake msg, parse its byte, and compare it to what we expect
* returns: error or nil
 */
func (t *PeerDownloader) receiveHandshakeMsg(hsk []byte, peer Peer) error {
	//parse and compare the version strlen
	pstrLen := int(hsk[0])
	if pstrLen != t.info.ProtoNameLen {
		return errors.New("receiveHandshakeMsg: pstrLen doesn't match")
	}
	//parse and compare the version string
	pstr := string(hsk[1 : pstrLen+1])
	if strings.Compare(pstr, t.info.ProtoName) != 0 {
		return errors.New("receiveHandshakeMsg: pstr doesn't match")
	}
	//parse and compare the info hash
	infoHash := string(hsk[pstrLen+9 : pstrLen+29])
	if strings.Compare(infoHash, t.info.InfoHash) != 0 {
		return errors.New("receiveHandshakeMsg: infoHasH doesn't match")
	}
	//parse and cmpare the peer id
	peerID := string(hsk[pstrLen+9+20:])
	if strings.Compare(peerID, peer.PeerID) != 0 {
		return errors.New("receiveHandshakeMsg: peerId doesn't match")
	}

	return nil

}

// StartDownload method starts download
func (t *PeerDownloader) StartDownload() error {

	for _, peerEntry := range t.peerList {
		conn, err := net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))
		if err != nil {
			return err
		}
		//create socket buffered writer
		pWriter := bufio.NewWriter(conn)
		//create socket buffered reader
		pRead := bufio.NewReader(conn)

		hskmsg := t.getHandshakeMsg()

		bytesWrite, err := pWriter.Write(hskmsg)
		if err != nil {
			return err
		} else if bytesWrite != len(hskmsg) {
			return errors.New("StartDownload: not all bytes could be written for handshake")
		} else if err = pWriter.Flush(); err != nil {
			return errors.New("StartDownload: could not write handshake to socket")
		}

		data, err := t.getHandshakePacket(pRead)
		if err != nil {
			log.Fatal(err)
		} else if err = t.receiveHandshakeMsg(data, peerEntry); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s", string(hskmsg))
	}

	return nil
}

func (t *PeerDownloader) getHandshakePacket(pRead *bufio.Reader) ([]byte, error) {
	// read 1 bytes to find out pstrlen
	pstrlen, err := pRead.ReadByte()
	if err != nil {
		return nil, errors.New("Could not read handhake pstr length")
	}
	length := int(pstrlen) + 48 // len += 8 unset bytes + 20 peer id + 20 infohash

	data, err := t.readPacket(length, pRead)
	data = append([]byte{pstrlen}, data...)
	return data, err
}

func (t *PeerDownloader) getPacket(pRead *bufio.Reader) ([]byte, error) {
	// read 1 bytes to find out length
	n, err := pRead.ReadByte()
	if err != nil {
		return nil, errors.New("Could not read packet length")
	}
	length := int(n)

	data, err := t.readPacket(length, pRead)
	data = append([]byte{n}, data...)
	return data, err
}

func (t *PeerDownloader) readPacket(length int, pRead *bufio.Reader) ([]byte, error) {
	data := make([]byte, length, length)
	readData := make([]byte, length, length)
	totalRead := 0
	for totalRead < length {
		nRead, err := pRead.Read(readData)
		if err != nil {
			return nil, errors.New("Could not read packet")
		}
		//fmt.Printf("%v\n", readData)
		data = append(data[:totalRead], readData...)
		totalRead += nRead
		fmt.Println(totalRead)
	}
	return data, nil
}
