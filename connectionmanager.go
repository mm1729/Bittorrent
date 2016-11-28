package main

/*
*managers sending and receiving messages to single peer
*opened on successful tcp connection to peer (incoming or outgoing)
 */

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

/*
* represents the current status of different components of the connection
 */
type ConnectionStatus struct {
	PeerChoked       bool //is the peer choking us?
	ClientChoked     bool //are we choking the peer?
	Downloading      bool //are we downloading?
	Uploading        bool //are we uploading
	Canceled         bool //idk what this is
	PeerInterested   bool //is  the peer  interested in us?
	ClientInterested bool //are we interested in the  peer  ?
}

type ConnectionManager struct {
	status        ConnectionStatus
	pieceManager  *PieceManager //the global piece manager
	msgQueue      [][]byte      //the queue of messages to be sent
	descriptor    int           //the handle used for piece manager for this connection
	packetHandler *Packet       //packet handle functions
	pWriter       *bufio.Writer //writer for  this connection
	pReader       *bufio.Reader //reader for this connection
	tInfo         TorrentInfo

	queueLock *sync.Mutex

	toPeerContact   chan<- bool //notifies peer contact manager if we unchoke a peer
	fromPeerContact <-chan bool //gives us peermission to unchoke a peer

	timeout int
	conn    net.Conn

	lastPieceRequest int
	mutex            *sync.Mutex

	flushChan chan bool
}

/*
* create a new peer connection struct
* @pieceManager: ptr toglobal piece manager for keeping track of pieces uploaded/downloaded
* @msgQueueMax: the max size of the queue
* returns: the new connection struct
 */
func NewConnectionManager(pieceManager *PieceManager, msgQueueMax int, out chan<- bool, in <-chan bool) ConnectionManager {
	var p ConnectionManager

	p.pieceManager = pieceManager
	//register a connection with the piecemanager so it can keep track of pieces

	p.msgQueue = make([][]byte, 0, msgQueueMax)

	p.status.PeerChoked = true
	p.status.ClientChoked = true
	p.status.Downloading = false
	p.status.Uploading = false
	p.status.Canceled = false
	p.status.PeerInterested = false
	p.status.ClientInterested = false

	p.toPeerContact = out
	p.fromPeerContact = in

	p.queueLock = &sync.Mutex{}
	p.mutex = &sync.Mutex{} // lock for lastrequest piece

	var pkt Packet

	p.packetHandler = &pkt

	p.lastPieceRequest = -1
	return p
}

func (t *ConnectionManager) StopConnection() {
	t.mutex.Lock()
	t.pieceManager.UnregisterConnection(t.descriptor, t.lastPieceRequest)

	t.pWriter.Flush()

	t.mutex.Unlock()
}

/*
* starts a handshake with the peer
* @conn: the tcp connection for this peer
* @peer: the peer info struct
* @tInfo: torent file information struct
* returns: error
 */
func (t *ConnectionManager) StartConnection(conn net.Conn, peer Peer, tInfo TorrentInfo, timeout int, interval int) error {

	ticker := time.NewTicker(time.Second * time.Duration(interval))
	t.flushChan = make(chan bool)
	go func(flushChan chan bool) {

		for _ = range ticker.C {
			t.flushChan <- true
		}
	}(t.flushChan)

	t.pWriter = bufio.NewWriter(conn)
	t.pReader = bufio.NewReader(conn)
	t.tInfo = tInfo
	t.timeout = timeout
	t.conn = conn

	if err := t.packetHandler.SendHandshakePacket(t.pWriter, tInfo); err != nil {
		return err
	}

	if err := t.packetHandler.ReceiveHandshakePacket(t.pReader, peer, tInfo); err != nil {
		return err
	}

	if err := t.sendBitFieldMessage(); err != nil {
		return err
	}

	if err := t.receiveBitFieldMessage(); err != nil {
		return err
	}

	return nil

}

/*
* sends a bitfield message to the peer
* retruns: error
 */
func (t *ConnectionManager) sendBitFieldMessage() error {
	//return our bitfield to the peer, to see if they are interested in us
	msg, err := CreateMessage(BITFIELD, NewPayload(BITFIELD, t.pieceManager.GetBitField()))
	if err != nil {
		return err
	}
	return t.packetHandler.SendArbitraryPacket(t.pWriter, msg)

}

/*
* receives a bitfield message from the peer and registers a connection with the piecemanager utilizing the received peer field
* returns: error
 */
func (t *ConnectionManager) receiveBitFieldMessage() error {
	//register out connection with the piecemanager by giving it our peer's bitfield
	inMessage, err := t.packetHandler.ReceiveArbitraryPacket(t.pReader, t.timeout, t.conn)

	if err != nil {

		return err
	}
	fmt.Printf("PEER FIELD %v\n", inMessage.Payload.bitField)
	t.descriptor = t.pieceManager.RegisterConnection(inMessage.Payload.bitField)
	//fmt.Println(inMessage.Payload.bitField)

	if t.pieceManager.ComputeRequestQueue(t.descriptor) == true {

		var msg []byte
		var err error

		if msg, err = CreateMessage(INTERESTED, Payload{}); err != nil {
			return err
		}

		if err := t.packetHandler.SendArbitraryPacket(t.pWriter, msg); err != nil {
			return err
		}
		t.status.ClientInterested = true
	} else {

		var msg []byte
		var err error
		if msg, err = CreateMessage(NOTINTERESTED, Payload{}); err != nil {
			return err
		}

		if err := t.packetHandler.SendArbitraryPacket(t.pWriter, msg); err != nil {
			return err
		}
		t.status.ClientInterested = false
	}
	return nil

}

/*
* determines what to do with a message
* @msg: Message struct
* returns: message to respond, error
 */
func (t *ConnectionManager) ReceiveNextMessage() error {

	inMessage, err := t.packetHandler.ReceiveArbitraryPacket(t.pReader, t.timeout, t.conn)

	if err != nil {

		return err
	}

	switch inMessage.Mtype {
	case KEEPALIVE:
		fmt.Println("KEEPALIVE")
		return nil
		//implement
		//clock how much time has gone by, then push a keepalive in
	case CHOKE:
		fmt.Println("CHOKE")
		//the peer has choked us
		t.status.PeerChoked = true
	case UNCHOKE:

		//the peer has unchoked us

		t.status.PeerChoked = false
	case INTERESTED:

		//peer is interested in downloading from us
		t.status.PeerInterested = true
		//request permission to unchoke this peer
		fmt.Println("INTERESTED")

		t.toPeerContact <- true
		if answer := <-t.fromPeerContact; answer == true {
			t.status.ClientChoked = false
			//permission granted, unchoke them
			if err := t.QueueMessage(UNCHOKE, Payload{}); err != nil {
				return err
			}
		} else {
			t.status.ClientChoked = true
			//maybe send a choke msg, or unchoke at a later time?
		}
	case NOTINTERESTED:
		fmt.Println("NOT INTERESTED")
		//peer is not interested in downloading from us
		t.status.PeerInterested = false
	case BITFIELD:
		fmt.Println("BITFIELD")
		//this would be an error
	case PIECE:
		fmt.Printf("CONNECTION %d: PIECE %d\n", t.descriptor, inMessage.Payload.pieceIndex)
		//received a piece from peer
		t.pieceManager.ReceivePiece(t.descriptor, inMessage.Payload.pieceIndex, inMessage.Payload.block)
		t.mutex.Lock()
		t.lastPieceRequest = -1
		t.mutex.Unlock()
		//return HAVE MESSAGE to all peers
		t.pieceManager.CreateHaveBroadcast(t.descriptor, inMessage.Payload.pieceIndex)

	case REQUEST:
		//a peer has requested a piece
		fmt.Println("REQUEST", inMessage.Payload)

		if t.status.ClientChoked == true {
			return errors.New("Peer is choked. Cannot cater requests from it")
		}

		if err, data := t.pieceManager.GetPiece(inMessage.Payload.pieceIndex); err != nil {
			//return piece response
			payload := Payload{inMessage.Payload.pieceIndex, []byte{}, 0, int32(len(data)), data}
			if err := t.QueueMessage(PIECE, payload); err != nil {
				return err
			}

		} else { // could not cater the request
			return err
		}

	case HAVE:
		fmt.Println("HAVE")
		//the peer is sending a have msg to update its bitfield
		t.pieceManager.UpdatePeerField(t.descriptor, inMessage.Payload.pieceIndex)

	case CANCEL:
		fmt.Println("CANCEL")
		//implement
	}

	// if we were not interested, we might be now
	if t.status.ClientInterested == false {
		if val := t.pieceManager.ComputeRequestQueue(t.descriptor); val == true {
			t.status.ClientInterested = val

			if err := t.QueueMessage(INTERESTED, Payload{}); err != nil {
				return err
			}

		}

	}

	//if we are interested in this client and not choked
	if t.status.ClientInterested == true && t.status.PeerChoked == false {
		//get next piece to download
		reqPieceID := t.pieceManager.GetNextRequest(t.descriptor)
		//	fmt.Println(reqPieceID)
		t.mutex.Lock()
		t.lastPieceRequest = reqPieceID
		t.mutex.Unlock()
		if reqPieceID == -1 {
			//	fmt.Printf("CONNECT %d, NOT\n", t.descriptor)
			if err := t.QueueMessage(NOTINTERESTED, Payload{}); err != nil {
				return err
			}
			t.status.ClientInterested = false

		} else {
			//send a request message for that piece, put in queue
			//fmt.Printf("Connection %d, REQUEST PIECE %d\n", t.descriptor, reqPieceID)
			if err := t.QueueMessage(REQUEST, Payload{pieceIndex: int32(reqPieceID), begin: 0, length: int32(t.tInfo.TInfo.PieceLength)}); err != nil {
				return err
			}

		}
	}

	return nil
}

func (t *ConnectionManager) QueueMessage(mType MsgType, payload Payload) error {
	var msg []byte
	var err error
	if msg, err = CreateMessage(mType, payload); err != nil {
		return err
	}

	t.queueLock.Lock()

	t.msgQueue = append(t.msgQueue, msg)
	t.queueLock.Unlock()

	return nil

}

func (t *ConnectionManager) SendNextMessage() error {
	//check if there are any have broadcasts

	select {

	case <-t.flushChan:
		channels := t.pieceManager.GetNextHaveBroadcast(t.descriptor)
		close(channels)
		for index := range channels {
			if err := t.QueueMessage(HAVE, Payload{pieceIndex: int32(index)}); err != nil {
				return err
			}

		}
	default:
	}
	t.queueLock.Lock()
	if len(t.msgQueue) == 0 {

		t.queueLock.Unlock()
		return nil
	}

	msg := t.msgQueue[0]
	t.msgQueue = t.msgQueue[1:]
	t.queueLock.Unlock()

	return t.packetHandler.SendArbitraryPacket(t.pWriter, msg)
}

func (t *ConnectionManager) GetConnectionStatus() ConnectionStatus {
	return t.status

}
