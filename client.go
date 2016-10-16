package main

import (
	"fmt"
	"log"
	"os"

	"github.com/amy/Bittorrent/torrent"
)

//announceURL+"?info_hash="+infoHash+"&peer_id="+peerId+"&peer_ip="+
//peerIp+"&port="+port+"&download="+download+"&left="+left+"&event="+event+"&no_peer_id=1"+"&compact=1"

func main() {

	if len(os.Args) != 2 {
		fmt.Println("Illegal USAGE!\n USAGE : ./Bittorrent <torrent_file>")
	}
	torrentFile := os.Args[1]

	torrent, err := torrent.NewTorrent(torrentFile)
	if err != nil {
		log.Fatal("Unable to decode the torrent file\n", err)
	}

	// create a new tracker and receive the list of peers
	hash := torrent.InfoHash()
	iDict := torrent.InfoDict()
	tkInfo := NewTracker(hash, torrent, &iDict)
	peerList := tkInfo.Connect()
	fmt.Printf("%v\n", peerList)
	tkInfo.Disconnect()
}
