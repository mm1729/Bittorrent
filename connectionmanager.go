package main

type ConnectionStatus struct {
	PeerChoked       bool
	ClientChoked     bool
	Downloading      bool
	Uploading        bool
	Canceled         bool
	PeerInterested   bool
	ClientInterested bool
}

type ConnectionManager struct {
	status       ConnectionStatus
	pieceManager *PieceManager
	msgQueue     []Message
}

func NewConnectionManager() ConnectionManager {
	var p ConnectionManager

}

func (t *ConnectionManager) StartConnection() {

}

func (t *ConnectionManager) ReceiveMessage(msg Message) (Message, error) {
	switch msg.MType {
	case KEEPALIVE:
	case CHOKE:
		//the peer has choked us
		t.status.PeerChoked = true
	case UNCHOKE:
		//the peer has unchoked us
		t.status.PeerChoked = false
	case INTERESTED:
		//peer is interested in downloading from us
		t.status.PeerInterested = true
		return CreateMessage(UNCHOKE, Payload{})
	case NOTINTERSTED:
		//peer is not interested in downloading from us
		t.status.PeerInterested = false
	case BITFIELD:
		//check if we are interested in downloading from this peer
		if t.pieceManager.CompareBitField(msg.Payload.bitField) == true {
			t.status.ClientInterested = true
		} else {
			//not interested
			t.status.ClientInterested = false
		}
		//return our bitfield to the peer, to see if they are interested in us
		return CreateMessage(BITFIELD, NewPayload(BITFIELD, t.pieceManager.GetBitField()))
	case PIECE:
		//received a piece from peer
		t.pieceManager.ReceivePiece(msg.Payload.pieceIndex, msg.Payload.block)
		//return HAVE MESSAGE to all peers

	case REQUEST:
		//a peer has requested a piece
		if t.pieceManager.GetPiece(msg.Payload.pieceIndex) == true {
			//create piece response message to upload to peer
			//return it

		} else {
			return Message{}, errors.New("Piece requested is not currently in possesion")
		
	case HAVE:
		//implement
		//if we weren't interested, and are now, send an interested message
		//else do nothing, the new piece has been added to the queue
	case CANCEL:
		//implement
	}

	if t.status.ClientInterested == true && t.status.PeerChoked == false {
		reqPieceID := t.pieceManager.GetNextRequest()
		if reqPieceID == -1 {
			t.status.Downloading == false
		}
	}
}

func (t *ConnectionManager) SendMessage(msg Message) error {

}

func (t *ConnectionManager) GetConnectionStatus() ConnectionStatus {
	return t.status

}
