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

	toPeerContact   chan<- bool //notifies peer contact manager if we unchoke a peer
	fromPeerContact <-chan bool //gives us peermission to unchoke a peer
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

	var pkt Packet

	p.packetHandler = &pkt
	return p
}

/*
* starts a handshake with the peer
* @conn: the tcp connection for this peer
* @peer: the peer info struct
* @tInfo: torent file information struct
* returns: error
 */
func (t *ConnectionManager) StartConnection(conn net.Conn, peer Peer, tInfo TorrentInfo) error {

	t.pWriter = bufio.NewWriter(conn)
	t.pReader = bufio.NewReader(conn)

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
	inMessage, err := t.packetHandler.ReceiveArbitraryPacket(t.pReader)
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", inMessage)
	t.descriptor = t.pieceManager.RegisterConnection(inMessage.Payload.bitField)
	fmt.Println("MADE IT")
	if t.pieceManager.ComputeRequestQueue(t.descriptor) == true {

		var msg []byte
		var err error

		if msg, err = CreateMessage(INTERESTED, Payload{}); err != nil {
			return err
		}
		fmt.Println("INTERESTED")
		if err := t.packetHandler.SendArbitraryPacket(t.pWriter, msg); err != nil {
			return err
		}
		t.status.ClientInterested = true
	} else {
		fmt.Println("NOT INTERESTED")
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
	inMessage, err := t.packetHandler.ReceiveArbitraryPacket(t.pReader)
	if err != nil {
		return err
	}
	switch inMessage.Mtype {
	case KEEPALIVE:
		//implement
		//clock how much time has gone by, then push a keepalive in
	case CHOKE:
		//the peer has choked us
		t.status.PeerChoked = true
	case UNCHOKE:
		//the peer has unchoked us
		t.status.PeerChoked = false
	case INTERESTED:
		//peer is interested in downloading from us
		t.status.PeerInterested = true
		//request permission to unchoke this peer
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
		//peer is not interested in downloading from us
		t.status.PeerInterested = false
	case BITFIELD:
		//this would be an error
	case PIECE:
		//received a piece from peer
		t.pieceManager.ReceivePiece(t.descriptor, inMessage.Payload.pieceIndex, inMessage.Payload.block)
		//return HAVE MESSAGE to all peers
		t.pieceManager.CreateHaveBroadcast(inMessage.Payload.pieceIndex)

	case REQUEST:
		//a peer has requested a piece
		if t.pieceManager.GetPiece(inMessage.Payload.pieceIndex) == true {
			//create piece response message to upload to peer
			//return it
			//implement

		} else {
			return errors.New("Piece requested is not currently in possesion")
		}

	case HAVE:
		//the peer is sending a have msg to update its bitfield
		t.pieceManager.UpdatePeerField(t.descriptor, inMessage.Payload.pieceIndex)

	case CANCEL:
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
		if reqPieceID == -1 {

			if err := t.QueueMessage(NOTINTERESTED, Payload{}); err != nil {
				return err
			}

		} else {
			//send a request message for that piece, put in queue

			if err := t.QueueMessage(REQUEST, Payload{pieceIndex: int32(reqPieceID), begin: 0, length: int32(t.tInfo.TInfo.PieceLength)}); err != nil {
				return err
			}

		}
	}
	//check if there are any have broadcasts
	if index := t.pieceManager.GetNextHaveBroadcast(t.descriptor); index != -1 {

		if err := t.QueueMessage(HAVE, Payload{pieceIndex: int32(index)}); err != nil {
			return err
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

	t.msgQueue = append(t.msgQueue, msg)
	return nil

}

func (t *ConnectionManager) SendNextMessage() error {
	msg := t.msgQueue[0]
	t.msgQueue = t.msgQueue[1:]
	return t.packetHandler.SendArbitraryPacket(t.pWriter, msg)
}

func (t *ConnectionManager) GetConnectionStatus() ConnectionStatus {
	return t.status

}
