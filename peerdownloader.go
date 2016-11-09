package main

import (
	"bufio"
	//"bytes"
	//"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	//"strings"
	"time"
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

// StartDownload method starts download
func (t *PeerDownloader) StartDownload() error {

	// find peer with min RTT
	peerEntry, rtt, err := t.findMinRTT(t.peerList)
	if err != nil {
		return err
	}
	fmt.Printf("\ndownloading from peer: %v rtt : %d\n", peerEntry, rtt)
	// 1.) make TCP connection
	conn, err := net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))
	if err != nil {
		return err
	}

	// 1.) make TCP connection
	conn, err = net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))
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
		return nil
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
	downloadTimeStart := time.Now()
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
		if err != nil {
			return err
		}
		if pieceMsg.Mtype == CHOKE {
			for {
				inMsg, err1 := t.getPacket(pRead)
				if err1 != nil {
					return err1
				}
				newMsg, err2 := NewMessage(inMsg)
				if err2 != nil {
					return err2
				}
				if newMsg.Mtype == UNCHOKE {
					break
				}
			}
		}

		// 12. Store the response
		t.manager.ReceivePiece(reqPieceID, pieceMsg.Payload.block)
		//fmt.Println(status)
	}
	fmt.Printf("Download Time :%v\n", time.Since(downloadTimeStart).String())
	return nil
}
