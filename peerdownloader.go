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
//Torrent info that peer downloader needs from the torrent file
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
func NewPeerDownloader(tInfo TorrentInfo, peers []Peer, fileName string) PeerDownloader {
	var p PeerDownloader

	p.info = tInfo
	p.peerList = peers
	p.manager = NewPieceManager(tInfo.TInfo, 10, fileName)

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

	// for each peer
	for _, peerEntry := range t.peerList {

		// 1.) make TCP connection
		conn, err := net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))
		if err != nil {
			return err
		}

		// 2.) get writer & reader to the connection
		//create socket buffered writer
		pWriter := bufio.NewWriter(conn)
		//create socket buffered reader
		pRead := bufio.NewReader(conn)

		// 3.) forms handshake message (i.e. what we can send) Written in []byte
		hskmsg := t.getHandshakeMsg()

		// 4.) send handshake message
		bytesWrite, err := pWriter.Write(hskmsg)
		if err != nil {
			return err
		} else if bytesWrite != len(hskmsg) {
			return errors.New("StartDownload: not all bytes could be written for handshake")
		} else if err = pWriter.Flush(); err != nil {
			return errors.New("StartDownload: could not write handshake to socket")
		}

		// 5.) gets handshake packet
		data, err := t.getHandshakePacket(pRead)
		if err != nil {
			log.Fatal(err)
			// validates handshake from peer. Check to make sure its the correct peer w/ info we want
		} else if err = t.receiveHandshakeMsg(data, peerEntry); err != nil {
			log.Fatal(err)
		}

		// 6.) send bitfield to peer. Indicates what pieces I have
		sendBits, err := t.sendBitField()
		bytesWrite, err = pWriter.Write(sendBits)
		if err != nil {
			return err
		} else if bytesWrite != len(sendBits) {
			return errors.New("StartDownload: not all bytes could be written for handshake")
		} else if err = pWriter.Flush(); err != nil {
			return errors.New("StartDownload: could not write handshake to socket")
		}

		// 7.) receive bitfield from peer. Indicates what pieces peer has.
		rawBitfield, err := t.getPacket(pRead)
		if err != nil {
			log.Fatal(err)
		}

		peerBitfield, err := t.receiveBitField(rawBitfield)
		if err != nil {
			log.Fatal(err)
		}

		if t.manager.CompareBitField(peerBitfield) == false {
			// maybe send not interested
			continue
		}

		// 8. Send Interested message
		message, err := CreateMessage(INTERESTED, Payload{})
		//fmt.Println(message)

		bytesWrite, err = pWriter.Write(message)
		fmt.Println()
		//fmt.Println("Request Msg: ",bytesWrite)
		if err != nil {
			return err
		} else if bytesWrite != len(message) {
			return errors.New("StartDownload: not all bytes could be written for handshake")
		} else if err = pWriter.Flush(); err != nil {
			return errors.New("StartDownload: could not write handshake to socket")
		}

		//9. Get an unchoke message
		_, err = t.getPacket(pRead)
		if err != nil {
			fmt.Println(err)
		}
		//fmt.Println(unchoke)

		for {
			//10. Send a request package
			reqPieceID := t.manager.GetNextRequest()
			fmt.Println("Requesting Piece: ", reqPieceID)
			if reqPieceID == -1 {
				break
			}
			reqMsg, err := t.getRequestMessage(int32(reqPieceID))
			bytesWrite, err = pWriter.Write(reqMsg)
			if err != nil {
				return err
			} else if bytesWrite != len(reqMsg) {
				return errors.New("StartDownload: not all bytes could be written for handshake")
			} else if err = pWriter.Flush(); err != nil {
				return errors.New("StartDownload: could not write handshake to socket")
			}

			// 11. Read the response
			piece, err := t.getPacket(pRead)
			if err != nil {
				return err
			}
			pieceMsg, err := NewMessage(piece)
			// 12. Store the response
			t.manager.ReceivePiece(reqPieceID, pieceMsg.Payload.block)
			//fmt.Println(status)
		}
	}

	return nil
}

func (t *PeerDownloader) getRequestMessage(pieceIndex int32) ([]byte, error) {
	reqPayload := Payload{pieceIndex: pieceIndex, begin: 0, length: int32(t.info.TInfo.PieceLength)}
	return CreateMessage(REQUEST, reqPayload)
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

// getPacket gets a TCP packet
// gets all TCP packets except handshake
func (t *PeerDownloader) getPacket(pRead *bufio.Reader) ([]byte, error) {
	// read 1 bytes to find out length

	msgLength := make([]byte, 4)
	msgLength[0], _ = pRead.ReadByte()
	msgLength[1], _ = pRead.ReadByte()
	msgLength[2], _ = pRead.ReadByte()
	msgLength[3], _ = pRead.ReadByte()

	var length int32
	binary.Read(bytes.NewReader(msgLength), binary.BigEndian, &length)

	data, err := t.readPacket(int(length), pRead)

	data = append(msgLength, data...)
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
		data = append(data[:totalRead], readData...)
		totalRead += nRead
	}
	return data, nil
}

// sendBitField is the bitfield I will send to peer
// gets bitfield from piecemanager
// returns bitfield message
func (t *PeerDownloader) sendBitField() ([]byte, error) {
	bitField := t.manager.GetBitField()

	payload := Payload{}
	payload.bitField = bitField

	return CreateMessage(BITFIELD, payload)
}

func (t *PeerDownloader) getProgress() (uploaded int, downloaded int, left int) {
	bitField := t.manager.GetBitField()
	uploaded = 0 // for now no uploading
	numDownloaded := 0
	for _, b := range bitField {
		if b != 0 {
			numDownloaded += int(((b >> 7) & 1) + ((b >> 6) & 1) + ((b >> 5) & 1) + ((b >> 4) & 1) + ((b >> 3) & 1) + ((b >> 2) & 1) + ((b >> 1) & 1) + b&1)
		}
	}
	downloaded = numDownloaded * t.info.TInfo.PieceLength
	left = t.info.TInfo.Length - downloaded
	return
}

// receiveBitField takes the message received
// and extracts the bitfield from it
func (t *PeerDownloader) receiveBitField(rawMsg []byte) ([]byte, error) {
	msg, err := NewMessage(rawMsg)
	if err != nil {
		log.Fatal("receiveBitField: receieved invalid raw message - ", err)
	}

	if msg.Mtype != BITFIELD {
		fmt.Println("MESSAGE")
		fmt.Println(msg)

		return nil, errors.New("Did not receive bitfield message")
	}

	return msg.Payload.bitField, nil
}
