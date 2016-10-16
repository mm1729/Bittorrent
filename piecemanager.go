package torrent




/*
 manages the pieces client needs to request
*/
type PieceManager struct{
	bitfield []byte 	//pieces client has
	missingField []byte	//missing 
	requestsQueue []int	//queues up index of pieces that client needs to request
	queueSize     int	//capacity of the requestQueue slice(chosen by user)
	fileWriter    *FileWriter
}



/*
 constructor
 @tInfo: contains information of about the torrent [pieceLength,length] see torrent.go
 @requestQueueSize: capacity for requestQueue slice [remains constant]
 returns: returns new PieceManager 
*/
func NewPieceManager( tInfo InfoDict,requestQueueSize int) (PieceManager){
	//create new piecemanager
	var p PieceManager

	//create file writer
	fW := NewFileWriter(tInfo)
	t.fileWriter = &fW

	//number of pieces in total
	numPieces := Math.Ceil(tInfo.length/tInfo.pieceLength) 
	//store the queue capacity
	t.queueSize  = requestQueueSize
	//number of bytes in bitField for client
	numBytes := Math.Ceil(numPieces/8)
	//create the bitfield with max numBytes
	p.bitfield =  make([]byte,numBytes)
	//make the requestqueue with the given the users capacity
	p.requestQueue = make([]int, requestQueueSize)

	return p
}


/*
 compare the peer's bitfield to ours and compute which ones we request from him
 @peerField: the peer's bitfield to compare against
 returns: whether client is interested
*/
func (t *PieceManager) CompareBitField(peerField []byte) bool{
	//initialize a missingField of capacity same as the peerField len
	t.missingField = make([]byte,cap(peerField))
	//default we are not interested
	interested := false
	//for all bytes in the peer field
	for index, element := peerField{
		//compute what we don't have that they have
		missingField[index] = ^(t.bitField[index]) &element
		//if they at least have something we want, we are interested
		if(missingField[index] !=0){
			interested = true
		}
	}
	return interested
	
}

/*
gets our bitField
returns: slice of bitField
*/
func (t *PieceManager) GetBitField() ([]byte) {
	return t.bitField
}


/*
*writes a received piece and marks it off and writes it
* @pieceIndex: piece we got
* @piece: the actual piece bytes
* returns: status
*/
func (t *PieceManager) ReceivedPiece(pieceIndex int, piece []byte) bool{
	bitmask := 1;
	nums := []int{0,1,2,3,4,5,6,7}
	//for all bytes in the missing field
	for index, element := t.missingField{
		//for all bits 
		for _,num := range nums{
			if(index*8+num == pieceIndex){
				//mark it off as zero
				t.missingField[index] = element & ^(bitmask <<num)
				//mark ours that we now have that piece
				t.bitField[index] = t.bitField[index] | (bitmask <<num)
				//write the piece
				t.fileWriter.Write(pieceIndex,piece)
				return true
			}
		}
	}
	return false

}

/*
 computes the Queue of requests that client makes
 returns: whether there are any left to request
*/
func  (t *PieceManager) computeQueue() bool{
	t.requestQueue = make([]int,t.queueSize)
	nums := []int{0,1,2,3,4,5,6,7}
	var bitmask byte
	//for all bytes in the missingField
	for index, element := t.missingField{
		//if this byte element is not 0
		if element != 0{
			bitmask = 1;
			//for all bits in this element
			for _,num := range nums{
				//if it is marked as 1, and we have room
				if elements & (bitmask << num) == 1 &&cap(t.requestQueue) != len(t.requestQueue){
					//append this index to the request queue
					t.reqeuestQueue = append(t.requestQueue,index*8+num)
				}else if cap(t.requestQueue)==len(t.requestQueue){
					return true;
				}
			}
		}
	}
 	//if we found nothing, we are done	
	if len(t.requestQueue) == 0{
		return false
	}else{
		return true
	}

}

/*
* gets the next piece to request,dequeues
* returns index
*/
func (t *PieceManager) GetNextRequest() int{
	//if queue is empty
	if len(t.requestQueue) == 0{
		//compute a new one if there is more to request
		if val := t.computeQueue(), val == -1{
			return -1;
		}
	}
	//pop off queue
	next := t.requestQueue[0]
	t.requestQueue = t.requestQueue[1:]
	return next
}

