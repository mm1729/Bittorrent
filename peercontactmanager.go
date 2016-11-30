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
	//	"log"
	"net"
	"strconv"
	"sync"
	//"strings"
	//	"time"
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
	wg             *sync.WaitGroup

	inComingChanListLock *sync.Mutex
	outGoingChanListLock *sync.Mutex
	outGoing             []chan bool
	inComing             []chan bool

	tracker *TrackerInfo
}

/*
NewPeerDownloader create a new peerdownloader
* @tInfo: torrent info dictionary
* @fileName: file to save pieces too
* @maxConnections: maximum TCP connections to peers (in or out) allowed)
* @maxUnchoked: maximum number of peers we can unchoke at once
* returns: new PeerDownloader
*/
func NewPeerContactManager(tracker *TrackerInfo, wg *sync.WaitGroup, tInfo TorrentInfo, fileName string, maxConnections uint32, maxUnchoked uint32, maxMsgQueue int) PeerContactManager {
	var p PeerContactManager
	p.wg = wg
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
	p.tracker = tracker
	go func(tracker *TrackerInfo) {
		status := p.pieceManager.WaitForDownload()
		for {
			select {
			case <-status:
				tracker.sendGetRequest("completed")
				fmt.Println("Download complete")
				return

			}

		}

	}(p.tracker)
	return p
}

/*

* given a list of peers to connect too, opens up outgoing connections up to maxConnections
* @peers: list of peers to contact, up to maxConnections
* returns: error
 */
func (t *PeerContactManager) StartOutgoing(peers []Peer) error {
	//handle the peer connection

	for _, peerEntry := range peers {
		// 1.) make TCP connection

		conn, err := net.Dial("tcp", peerEntry.IP+":"+strconv.FormatInt(peerEntry.Port, 10))

		if err != nil {
			return err
		}
		//spawn routine to handle connection
		t.wg.Add(1)

		go t.handler(conn, peerEntry)

	}
	t.wg.Wait()

	return nil

}

func (t *PeerContactManager) handler(tcpConnection net.Conn, peer Peer) {
	fmt.Printf("connection to %v spawned\n", peer.IP)

	//open up a new connection manager
	manager := NewConnectionManager(&t.pieceManager, t.msgQueueMax, t.in, t.out)
	//start up the connection
	if err := manager.StartConnection(tcpConnection, peer, t.tInfo, 120, 2); err != nil {

		//fmt.Printf("Failed to connect to %v: %v\n", tcpConnection.RemoteAddr(), err)
		tcpConnection.Close()
		t.wg.Done()
		return
	}
	//loop receiving and sending messages
	//send loop ( this might possibly speed things up

	errChan := make(chan error)
	go func(errChan chan error) {
		for {
			err := manager.SendNextMessage()
			if err != nil {
				errChan <- err
				return
			}

			select {
			case <-errChan:
				return
			default:
			}

		}
	}(errChan)
	//receive loop
	for {
		err := manager.ReceiveNextMessage()
		if err != nil {
			errChan <- err
			break
		}
		select {

		case <-errChan:
			break
		case <-t.in: //just unchoke all peers
			t.out <- true
		default:
		}
	}

	manager.StopConnection()
	tcpConnection.Close()
	t.wg.Done()

}
func (t *PeerContactManager) GetProgress() (int, int, int) {
	return t.pieceManager.GetProgress()
}

func (t *PeerContactManager) StopDownload() error {
	// Stop go functions here ?

	return t.pieceManager.SaveProgress()
}

/*
* opens up a listener to listen for incoming peer connections
* @port to openup listener on
* returns error
 */
func (t *PeerContactManager) StartIncoming(port uint32) error {
	// listen on all network interfaces on port input
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()

		if err != nil {
			return err
		}
		t.wg.Add(1)
		go t.incomingHandler(conn)

	}

	return nil
}

func (t *PeerContactManager) incomingHandler(conn net.Conn) {
	fmt.Println(conn.LocalAddr().String(), " Got connection from ", conn.RemoteAddr().String())
	t.handler(conn, Peer{})

}
