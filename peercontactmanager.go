package main

/*
manages connection to multiple peers simultaneously
responsible for manager incoming/outgoing connections
*/

import (
	//"bufio"
	//"bytes"
	//"encoding/binary"
	//"errors"
	"fmt"
	//"log"
	"net"
	"strconv"
	//"strings"
	//"time"
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
type PeerContactManager struct {
	tInfo          TorrentInfo  //information about the torrent [see above]
	pieceManager   PieceManager //manages requests for pieces
	maxConnections uint32
	maxUnchoked    uint32
	in             chan bool
	out            chan bool
	msgQueueMax    int //maxmimum number of pieces queue up for a peer
}

/*
NewPeerDownloader create a new peerdownloader
* @tInfo: torrent info dictionary
* @fileName: file to save pieces too
* @maxConnections: maximum TCP connections to peers (in or out) allowed)
* @maxUnchoked: maximum number of peers we can unchoke at once
* returns: new PeerDownloader
*/
func NewPeerContactManager(tInfo TorrentInfo, fileName string, maxConnections uint32, maxUnchoked uint32, maxMsgQueue int) PeerContactManager {
	var p PeerContactManager

	p.tInfo = tInfo
	//global manager for pieces we have and need
	p.pieceManager = NewPieceManager(tInfo.TInfo, 10, fileName)
	//number of peers allowed to be connected to simultaneously
	p.maxConnections = maxConnections
	//number of peers we are allowed to unchoke
	p.maxUnchoked = maxUnchoked

	p.in = make(chan bool)  //receive requests for unchoking
	p.out = make(chan bool) //respond to requests for unchoking
	p.msgQueueMax = maxMsgQueue
	return p
}

/*

* given a list of peers to connect too, opens up outgoing connections up to maxConnections
* @peers: list of peers to contact, up to maxConnections
* returns: error
 */
func (t *PeerContactManager) StartOutgoing(peers []Peer) error {
	//handle the peer connection
	handler := func(tcpConnection net.Conn, peer Peer) {
		fmt.Printf("connection to %v spawned\n", peer.IP)

		//open up a new connection manager
		manager := NewConnectionManager(&t.pieceManager, t.msgQueueMax, t.in, t.out)
		//start up the connection
		manager.StartConnection(tcpConnection, peer, t.tInfo)
		//loop receiving and sending messages
		go func() {
			for {

				manager.ReceiveNextMessage()
			}
		}()
		for {
			manager.SendNextMessage()
		}

	}

	for _, peerEntry := range peers {
		// 1.) make TCP connection
		fmt.Printf("ATTEMPTING %v\n", peerEntry.IP)
		conn, err := net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))
		fmt.Printf("YES %v\n", peerEntry.IP)
		if err != nil {
			return err
		}
		//spawn routine to handle connection
		go handler(conn, peerEntry)

	}

	return nil

}

/*
* opens up a listener to listen for incoming peer connections
* @port to openup listener on
* returns error
 */
func (t *PeerContactManager) StartIncoming(port uint32) error {
	return nil
}
