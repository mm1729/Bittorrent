Amy Chen
Moulindra Muchumari
Dylan Herman

IMPLEMENTATION DETAILS

Dependency Diagram:


 Client --GET list of peers-> Tracker---Contact Tracker--> Tracker server 
    |
    --peers list---> PeerDownloader---get bittorrent packets--> Message
                                |
                                 --send/get pieces--> PieceManager --write data-> FileWriter --write to disk-> I/O disk   
 
 
///////////////////////////////// 
// Download & Upload Bitfields //
/////////////////////////////////

Per peer, PeerDownload contacts all peers. 

> Handshakes Peer: client tells peer its client ID & torrent hash
> Handshake from Peer: peer sends its ID and torrent hash
> SUCCESS: if handshake is successful, EXCHANGE BITFIELDS

Exchange Bitfields: 
A bitfield represents the pieces that a peer has. The peer 
and client exchange bitfields. On the client, we have 2 byte
slices. One slice represents the bits we have received (aka
currently own). The other slice represents pieces we are missing
(aka pieces we need to request from the peer).
    
    our pieces      [0 0 0 0 0 0]
    missing pieces  [1 1 1 1 1 1]
    
When we receive a piece, we flip the bit for the ones we receive. 
To calculate the missing: !(our pieces) & (peer pieces) = (missing pieces)

Because our bitfields are represented in byte slices, the following 
calculation is needed:

    PIECE INDEX = (byte index * 8) + bit index

//////////////////////////////
// DOWNLOAD & UPLOAD PIECES //
//////////////////////////////

If PieceManager determines we need a piece, it returns TRUE from 
compareBitField. Otherwise we move on to the next peer.

We send the peer an INTERESTED message to notify we would like to 
download a piece(s). 

We wait for an UNCHOKE message from the peer, which indicates that 
the peer is ready to send pieces. After receiving the unchoke message, 
our connection has state UNCHOKED and INTERESTED. At this point, we 
are able to send request messages and receive piece messages as response. 

Next, we ask PieceManager for the piece index to be requested. 
Then, we send the REQUEST message to the peer. After sending the request,
we wait for a PIECE message for the REQUEST sent. We currently only 
send one REQUEST at a time and wait until the corresponding PIECE message
is received to the send the next REQUEST. If the peer receives a CHOKE
message at any time in this loop, we wait until we receive an UNCHOKE 
message to resume sending REQUESTS.

//////////////////
// PIECEMANAGER //
//////////////////


PieceManager keeps track of what pieces need to be downloaded. Currently,
it works with one peer at a time. It stores the peer bitfield and the
client bitfield to check what pieces it is missing. PieceManager stores
the requests (10 by default) in a queue to quickly send the requests