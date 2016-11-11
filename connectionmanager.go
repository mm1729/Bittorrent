package main

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
	packetHandler PacketHandler //packet handle functions
	pWriter       bufio.Writer  //writer for  this connection
	pReader       bufio.Reader  //reader for this connection
	tInfo         TorrentInfo
}

/*
* create a new peer connection struct
* @pieceManager: ptr toglobal piece manager for keeping track of pieces uploaded/downloaded
* @msgQueueMax: the max size of the queue
* returns: the new connection struct
 */
func NewConnectionManager(pieceManager *PieceManager, msgQueueMax int) ConnectionManager {
	var p ConnectionManager
	p.pieceManager = pieceManager
	//register a connection with the piecemanager so it can keep track of pieces
	p.descriptor = pieceManager.RegisterConnection()

	p.msgQueue = make([]Message, 0, msgQueueMax)

	p.status.PeerChoked = true
	p.status.ClientChoked = true
	p.status.Downloading = false
	p.status.Uploading = false
	p.status.Canceled = false
	p.status.PeerInterested = false
	p.status.ClientInterested = false

	var pkt Packet

	p.packetHandler = pkt

	return p
}

/*
* starts a handshake with the peer
* @conn: the tcp connection for this peer
* @peer: the peer info struct
* @tInfo: torent file information struct
* returns: error
 */
func (t *ConnectionManager) StartConnection(conn Conn, peer Peer, tInfo TorrentInfo) error {

	p.pWriter = bufio.NewWriter(conn)
	p.pReader = bufio.NewReader(conn)

	if err := p.packetHandler.SendHandshakePacket(p.pWriter, tInfo); err != nil {
		return err
	}

	if err = p.packetHandler.ReceiveHandshakePacket(p.pReader, peer, tInfo); err != nil {
		return err
	}

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
	switch msg.MType {
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
		//unchoke them
		t.msgQueue = append(t.msgQueue, CreateMessage(UNCHOKE, Payload{}))
	case NOTINTERSTED:
		//peer is not interested in downloading from us
		t.status.PeerInterested = false
	case BITFIELD:
		//register out connection with the piecemanager by giving it our peer's bitfield
		t.descriptor = t.pieceManager.RegisterConnection(msg.Payload.bitField)

		//return our bitfield to the peer, to see if they are interested in us
		t.msgQueue = append(t.msgQueue, CreateMessage(BITFIELD, NewPayload(BITFIELD, t.pieceManager.GetBitField())))

		if t.pieceManager.ComputeRequestQueue(t.descriptor) == true {
			t.msgQueue = append(t.msgQueue, CreateMessage(INTERESTED, Payload{}))
			t.ClientInterested = true
		} else {
			t.msgQueue = append(t.msgQueue, CreateMessage(NOTINTERESTED, Payload{}))
			t.ClientInterested = false
		}

	case PIECE:
		//received a piece from peer
		t.pieceManager.ReceivePiece(msg.Payload.pieceIndex, msg.Payload.block)
		//return HAVE MESSAGE to all peers
		t.pieceManager.CreateHaveBroadcast(msg.Payload.pieceIndex)

	case REQUEST:
		//a peer has requested a piece
		if t.pieceManager.GetPiece(msg.Payload.pieceIndex) == true {
			//create piece response message to upload to peer
			//return it
			//implement

		} else {
			return Message{}, errors.New("Piece requested is not currently in possesion")
		}

	case HAVE:
		//the peer is sending a have msg to update its bitfield
		t.pieceManager.UpdatePeerField(t.descriptor, msg.Payload.pieceIndex)

	case CANCEL:
		//implement
	}

	// if we were not interested, we might be now
	if t.status.ClientInterested == false {
		if val := t.pieceManager.ComputeRequestQueue(t.descriptor); val == true {
			t.status.ClientInterested = val
			t.msgQueue = append(t.msgQueue, CreateMessage(INTERESTED, Payload{}))

		}

	}

	//if we are interested in this client and not choked
	if t.status.ClientInterested == true && t.status.PeerChoked == false {
		//get next piece to download
		reqPieceID := t.pieceManager.GetNextRequest()
		if reqPieceID == -1 {

			t.msgQueue = append(t.msgQueue, CreateMessage(NOTINTERESTED, Payload{}))
		} else {
			//send a request message for that piece, put in queue

			t.msgQueue = append(t.msgQueue, CreateMessage(REQUEST, Payload{pieceIndex: rePieceID, begin: 0, length: int32(t.tInfo.TInfo.PieceLength)}))
		}
	}
	//check if there are any have broadcasts
	if index := t.pieceManager.GetNextHaveBroadcast(t.descriptor); index != -1 {
		t.msgQueue = append(t.msgQueue, CreateMessage(HAVE, Payload{pieceIndex: index}))
	}
	return nil
}

func (t *ConnectionManager) SendNextMessage() error {
	msg := t.msgQueue[0]
	t.msgQueue = t.msgQueue[1:]
	return t.packet.SendArbitraryPacket(t.pWriter, msg)
}

func (t *ConnectionManager) GetConnectionStatus() ConnectionStatus {
	return t.status

}
