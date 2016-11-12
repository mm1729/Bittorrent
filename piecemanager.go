package main

/*

* matains state of which pieces we have and what pieces we need
* accessed by multiple peer connections for piece requests
 */

import (
	"errors"
	"fmt"
	"math"
	"sync"
	//"os"
)

type HaveBroadcast struct {
	pieceIndex int32
	notSeenBy  int
}

/*
* manages pieces for a single peer connection
 */
type ConnectionPieceManager struct {
	requestQueue   []int  //holds the next pieces to request
	peerField      []byte //pieces the peer has
	mostRecentHave int64  //last seen have message

}

/*
PieceManager manages the pieces client needs to request
*/
type PieceManager struct {
	//global fields
	bitField []byte //pieces client has
	//missingField []byte //missing
	transitField []byte //determine if piece is intransit

	maxQueueSize int //capacity of the requestQueue slice(chosen by user)

	manager        []*ConnectionPieceManager //manages piece queues for a given peer
	numConnections int

	fileWriter *FileWriter
	infoDict   *InfoDict

	haveQueue     []HaveBroadcast //broadcast which pieces we havei
	lastHaveIndex uint64

	mutex *sync.Mutex
}

/*
NewPieceManager constructor
 @tInfo: contains information of about the torrent [pieceLength,length] see torrent.go
 @requestQueueSize: capacity for requestQueue slice [remains constant]
 returns: returns new PieceManager
*/
func NewPieceManager(tInfo *InfoDict, requestQueueSize int, fileName string) PieceManager {
	//create new piecemanager
	var p PieceManager

	//create file writer
	fW := NewFileWriter(tInfo, fileName)
	p.fileWriter = &fW

	//number of pieces in total
	numPieces := math.Ceil(float64(tInfo.Length) / float64(tInfo.PieceLength))
	//store the request queue capacity
	p.maxQueueSize = requestQueueSize
	//number of bytes in bitField for client
	numBytes := math.Ceil(numPieces / 8)
	//create the bitfield with max numBytes
	p.bitField = make([]byte, int(numBytes), int(numBytes))
	//pieces which peers have claimed responsbility
	p.transitField = make([]byte, int(numBytes), int(numBytes))

	p.numConnections = 0
	p.lastHaveIndex = 0
	p.infoDict = tInfo

	p.mutex = &sync.Mutex{}

	return p
}

func (t *PieceManager) LoadBitFieldFromFile() {
	//implement
}

/*
* create a piece manager for a new connection
* returns: connection descriptor
 */
func (t *PieceManager) RegisterConnection(peerField []byte) int {
	conNum := t.numConnections
	t.numConnections++

	var con ConnectionPieceManager
	t.manager = append(t.manager, &con)
	con.mostRecentHave = -1
	con.peerField = make([]byte, cap(t.bitField), cap(t.bitField))
	copy(con.peerField, peerField)
	return conNum
}

func (t *PieceManager) UnregisterConnection(connection int) {
	for _, index := range t.manager[connection].requestQueue {
		byteIndex := index / 8
		offset := uint32(index % 8)

		t.transitField[byteIndex] &= (0 << (7 - offset))
	}
}

/**
* used if a peer sends a have message updating us a of new piece they have
* check if we either have it or another peer is offering it
* @connection: the connection descriptor for this peer
* @pieceIndex: the actual index of this piece
* returns: whether we are interested in it
**/
func (t *PieceManager) UpdatePeerField(connection int, pieceIndex int32) {
	//a peer has told us it now has a piece
	//are we now interested in it?
	//if we are interested add it to missingfield

	//compute the location of this piece in the bitfields
	index := pieceIndex / 8
	offset := uint32(pieceIndex % 8)
	bit := byte(1 << (7 - offset))

	//add to the peer's list of pieces they have
	t.manager[connection].peerField[index] |= bit

}

/*
 *determines which pieces we should request from 'peer' using 'connecton'
 @connection: connection descriptor for the peer
 returns: whether client is interested
*/
func (t *PieceManager) ComputeRequestQueue(connection int) bool {
	//construct the new request queue for the peer
	t.manager[connection].requestQueue = make([]int, 0, t.maxQueueSize)
	//we are not interested by default
	interested := false

	t.mutex.Lock()

	//for all bytes in the peer field
	for index, element := range t.manager[connection].peerField {
		//compute what this peer has that no other peers has and we don't have
		mask := (^(t.bitField[index]) & element & ^(t.transitField[index]))
		//if there is anything found
		if mask != 0 {
			//we are interested
			interested = true
			nums := [8]uint32{0, 1, 2, 3, 4, 5, 6, 7}
			//go through the mask and get the index of those pieces
			for _, num := range nums {
				bit := byte(1 << (7 - num))
				if mask&bit != 0 {
					//if can fit in queue, add them to the queue
					if len(t.manager[connection].requestQueue) < t.maxQueueSize {
						//add piece to request queue
						t.manager[connection].requestQueue = append(t.manager[connection].requestQueue, index*8+int(num))
						//a peer has claimed responsibility for this piece
						t.transitField[index] |= 1 << (7 - num)
					}
				}
				//if we can no longer fit pieces in the queue
				if len(t.manager[connection].requestQueue) == t.maxQueueSize {
					t.mutex.Unlock()
					fmt.Printf("CONNECTION %d, QUEUE %v\n", connection, t.manager[connection].requestQueue)
					return interested
				}
			}

		}

	}
	t.mutex.Unlock()

	return interested

}

/*
GetBitField gets our bitField
returns: slice of bitField
*/
func (t *PieceManager) GetBitField() []byte {
	return t.bitField
}

/*
* checks to see if we have the piece requested from us
* @pieceIndex: index of piece to look for
* returns: whether we have it
 */
func (t *PieceManager) GetPiece(pieceIndex int32) bool {
	//implement
	index := pieceIndex / 8
	offset := uint32(pieceIndex % 8)
	bit := byte(1 << (7 - offset))
	if t.bitField[index]&bit == 0 {
		return false
	}
	//we have it, so fetch the piece
	return true
}

/*
ReceivedPiece writes a received piece and marks it off and writes it
* @pieceIndex: piece we got
* @piece: the actual piece bytes
* returns: status
*/
func (t *PieceManager) ReceivePiece(connection int, pieceIndex int32, piece []byte) error {

	index := pieceIndex / 8
	offset := uint32(pieceIndex % 8)
	bit := byte(1 << (7 - offset))
	if t.bitField[index]&bit == 1 {
		return errors.New("ReceivePiece: received piece we already have")
	} else {
		t.mutex.Lock()
		//we now have  the piece
		t.bitField[index] |= bit
		t.mutex.Unlock()

		err := t.fileWriter.Write(piece, int(pieceIndex))

		if err != nil {
			return errors.New("ReceivePiece: failed to write file")
		}

	}
	return nil

}

/*
* gets the next piece request for the given connection
* @connection: descriptor for the given connection
* returns index
 */
func (t *PieceManager) GetNextRequest(connection int) int {
	//if queue is empty
	if len(t.manager[connection].requestQueue) == 0 {
		//compute a new one if there is more to request
		if val := t.ComputeRequestQueue(connection); val == false {
			return -1
		}
	}
	//pop off queue
	next := t.manager[connection].requestQueue[0]
	t.manager[connection].requestQueue = t.manager[connection].requestQueue[1:]
	return next
}

/*
* attempt to retreive the next have broadcast, if there is one
* @connection: connection descriptor for the calling peer
* returns: the index of the piece broadcasted
 */
func (t *PieceManager) GetNextHaveBroadcast(connection int) int {
	/*//lock
	lastSeen := t.manager[connection].mostRecentHave
	if len(t.haveQueue) != 0 {
		for index := 0; index < len(t.haveQueue); index++ {
			if lastSeen < t.haveQueue[index].index {
				t.manager[connection].mostRecentHave = t.haveQueue[index].index
				t.haveQueue[index].notSeenBy--
				have := t.haveQueue[index].pieceIndex
				if t.haveQueue[index].notSeenBy == 0 {
					t.haveQueue = t.haveQueue[1:]
				}
				return have
			}
		}
	}*/
	return -1
	//unlock

}

/*
* creates a have broadcast to all connected peers when we get a new piece
* @pieceIndex: index to broadcast a notification of
 */
func (t *PieceManager) CreateHaveBroadcast(pieceIndex int32) {
	/*
		//lock
		var h HaveBroadcast
		h.pieceIndex = pieceIndex            //set the index of piece to broadcast
		t.lastHaveIndex += 1                 //increase the piecemanager's have index
		h.Index = (t.lastHaveIndex)     //set this have broadcast's index to the next monotically increasing index
		h.notSeenBy = t.numConnections       //keep a reference count of how many peers have seen this
		t.haveQueue = append(t.haveQueue, h) //append it to the queue
		//unlock*/
}

/**
* returns the current progress of the uploading/downloading
**/
func (t *PieceManager) GetProgress() (uploaded int, downloaded int, left int) {
	bitField := t.GetBitField()
	uploaded = 0 // for now no uploading
	numDownloaded := 0
	for _, b := range bitField {
		if b != 0 {
			numDownloaded += int(((b >> 7) & 1) + ((b >> 6) & 1) + ((b >> 5) & 1) + ((b >> 4) & 1) + ((b >> 3) & 1) + ((b >> 2) & 1) + ((b >> 1) & 1) + b&1)
		}
	}
	downloaded = numDownloaded * t.infoDict.PieceLength
	left = t.infoDict.Length - downloaded
	return
}
