package main

import (
	"fmt"
	"os"

	"github.com/amy/Internet-Technology/bitTorrent/torrent"
)

//announceURL+"?info_hash="+infoHash+"&peer_id="+peerId+"&peer_ip="+
//peerIp+"&port="+port+"&download="+download+"&left="+left+"&event="+event+"&no_peer_id=1"+"&compact=1"

func main() {

	torrentFile := os.Args[1]
	//destination := os.Args[2]

	// open torrent
	/*f, err := os.Open(torrentFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	dec := bencode.NewDecoder(f)*/

	torrent, _ := torrent.NewTorrent(torrentFile)

	/*if err := dec.Decode(&torrent); err != nil {
		panic(err)
	}*/

	//fmt.Printf("ANNOUNCE: %v\n", torrent.Announce)
	fmt.Printf("INFOHASH %v\n", torrent.InfoHash())

	//fmt.Printf("DICT: %d", torrent.Info)

	//temp := fmt.Sprintf("%x", torrent.Info)

	//fmt.Println(temp)

	//fmt.Printf("PIECES: %v\n", torrent.Info.Pieces)
	/*fmt.Printf("PIECE LENGTH: %v\n", torrent.Info.PieceLength)
	fmt.Printf("NAME: %v\n", torrent.Info.Name)
	fmt.Printf("LENGTH: %v\n", torrent.Info.Length)*/

	//fmt.Printf("FILES: %v\n", torrent.Info.Files)

}
