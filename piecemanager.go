package main

import (
	//	"fmt"
	"math"
	//"os"
)

/*
* manages pieces for a single peer connection
 */
type ConnectionPieceManager struct {
	requestQueue []int

	missingField []byte
}

/*
PieceManager manages the pieces client needs to request
*/
type PieceManager struct {
	//global fields
	bitField     []byte //pieces client has
	missingField []byte //missing
	maxQueueSize int    //capacity of the requestQueue slice(chosen by user)

	managers       []*ConnectionPieceManager //manages piece queues for a given peer
	numConnections int

	fileWriter *FileWriter
	infoDict   *InfoDict
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
	//create missing field to keep track of what pieces will be requested (don't request twice)
	p.missingField = make([]byte, cap(peerField), cap(peerField))

	p.numConnections = 0

	p.infoDict = tInfo
	return p
}

/*
* create a piece manager for a new connection
* returns: connection descriptor
 */
func (t *PieceManager) RegisterConnection() int {
	conNum = t.numConnections
	t.numConnections++

	var con ConnectionPieceManager
	t.manager[conNum] = &con

	con.missingField = make([]byte, cap(t.missingField), cap(t.missingField))
	return conNum
}

/**
* used if a peer sends a have message updating us a of new piece they have
* check if we either have it or another peer is offering it
* @connection: the connection descriptor for this peer
* @pieceIndex: the actual index of this piece
* returns: whether we are interested in it
**/
func (t *PieceManager) UpdateMissingField(connection int, pieceIndex int) bool {
	//a peer has told us it now has a piece
	//are we now interested in it?
	//if we are interested add it to missingfield

	//compute the location of this piece in the bitfields
	index := pieceIndex / 8
	offset := pieceIndex % 8
	bit = 1 << (7 - offset)

	mask := ^(t.bitField[index] & bit) & ^(t.missingField[index] && bit)
	//equals zero if we don't currently have it and it is not current offered by another peer we are talking too
	if mask == 0 {
		t.manager[connection].missingField[index] |= bit
		return true
	} else {
		return false
	}
}

/*
 *determines which pieces we should request from 'peer' using 'connecton'
 @peerField: the peer's bitfield
 @connection: connection descriptor for the peer
 returns: whether client is interested
*/
func (t *PieceManager) ComputeMissingField(peerField []byte, connection int) bool {
	//initialize a missingField of capacity same as the peerField len
	//default we are not interested
	interested := false
	//for all bytes in the peer field
	for index, element := range peerField {
		//compute what this peer has that we don't have and other peers we are talking to don't have
		mask := (^(t.bitField[index]) & element & ^(t.missingField[index]))
		//if there is anything found
		if mask != 0 {
			//we are interested
			interested = true
			//or it with the current missing field
			t.missingField[index] |= mask
			//add this to connection field
			t.manager[connection].missingField[index] = mask
		}

	}
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
func (t *PieceManager) GetPiece(pieceIndex int) bool {
	//implement
	index := pieceIndex / 8
	offset := pieceIndex % 8
	bit := 1 << (7 - offset)
	if t.bitField[index]&bit == 0 {
		return false
	}
	//we have it, so fetch the piece

}

//NOTE: FIX to allow for partial messages, what does it mean to have a partial message?
/*
ReceivedPiece writes a received piece and marks it off and writes it
* @pieceIndex: piece we got
* @piece: the actual piece bytes
* returns: status
*/
func (t *PieceManager) ReceivePiece(connection int, pieceIndex int, piece []byte) error {

	index := pieceIndex / 8
	offset := pieceIndex % 8
	bit := 1 << (7 - offset)
	if t.bitField[index]&bit == 1 || ^t.missingField[index]&bit == 1 {
		return errors.New("ReceivePiece: received piece we already have")
	} else {
		//we now have  the piece
		t.bitField[index] |= bit
		//remove it from the global missing field
		t.missingField[index] & & ^bit
		//remove it from `connection`'s missingField
		t.manager[connection].missingField[index] &= ^bit

		err := t.fileWriter.Write(piece, pieceIndex)

		if err != nil {
			return errors.New("ReceivePiece: failed to write file")
		}
		err = t.fileWriter.Sync()
		if err != nil {
			return errors.New("ReceivePiece: failed to write to disk")
		}

	}
	return nil

}

/*
* computes the Queue of requests that client makes
* @connection: connection descriptor for peer
* returns: whether there are any left to request
 */
func (t *PieceManager) computeQueue(connection int) bool {
	t.manager[connection].requestQueue = make([]int, 0, t.maxQueueSize)

	nums := [8]uint{0, 1, 2, 3, 4, 5, 6, 7}
	var bitmask byte
	//for all bytes in the missingField
	for index, element := range t.manager[connection].missingField {
		//if this byte element is not 0
		if element != 0 {
			bitmask = 1
			//for all bits in this element

			for _, num := range nums {
				//if it is marked as 1, and we have room
				if element&(bitmask<<(7-num)) != 0 && cap(t.manager[connection].requestQueue) != len(t.manager[connection].requestQueue) {

					//append this index to the request queue
					t.manager[connection].requestQueue = append(t.manager[connection].requestQueue, index*8+int(num))

				} else if cap(t.manager[connection].requestQueue) == len(t.manager[connection].requestQueue) {
					return true
				}
			}
		}
	}
	//if we found nothing, we are done
	if len(t.manager[connection].requestQueue) == 0 {
		return false
	}
	return true
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
		if val := t.computeQueue(connection); val == false {
			t.fileWriter.Sync()
			t.fileWriter.Finish()
			return -1
		}
	}
	//pop off queue
	next := t.manager[connection].requestQueue[0]
	t.requestQueue = t.manager[connection].requestQueue[1:]
	return next
}

func (t *PieceManager) getProgress() (uploaded int, downloaded int, left int) {
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
