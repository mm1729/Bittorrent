package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

//ClientID is the 20 byte id of our client
var ClientID = "DONDESTALABIBLIOTECA"

//ProtoName is the BitTorrent protocol we are using
var ProtoName = "BitTorrent protocol"

func main() {
	runtime.GOMAXPROCS(2)
	if len(os.Args) < 3 {
		fmt.Println("Illegal USAGE!\n USAGE : ./Bittorrent <torrent_file> <output file>")
		return
	}
	torrentFile := os.Args[1]
	fileName := os.Args[2]

	torrent, err := NewTorrent(torrentFile)
	if err != nil {
		log.Fatal("Unable to decode the torrent file\n", err)
	}

	// create a new tracker and receive the list of peers
	hash := torrent.InfoHash()
	iDict := torrent.InfoDict()

	// Tracker connection
	tkInfo := NewTracker(hash, torrent, &iDict)
	peerList, interval := tkInfo.Connect()
	fmt.Printf("%v\n", peerList)

	//Start peer download
	tInfo := TorrentInfo{
		TInfo:        &iDict,
		ClientID:     ClientID,
		ProtoName:    ProtoName,
		ProtoNameLen: len(ProtoName),
		InfoHash:     string(hash[:len(hash)]),
	}

	manager := NewPeerContactManager(tInfo, fileName, 10, 10, 10)
	//	tkInfo.sendGetRequest("")
	if err := manager.StartOutgoing(peerList); err != nil {
		fmt.Println("ERROR!\n")
		return
	}

	// keep announcing to tracker at Interval seconds
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	//fmt.Println("Sending updates to tracker at ", interval, "s")

	go func() {
		for _ = range ticker.C {
			tkInfo.Uploaded, tkInfo.Downloaded, tkInfo.Left =
				manager.GetProgress()
			tkInfo.sendGetRequest("")
		}
	}()
	for {
	}

	ticker.Stop() // ticker is done
	// Send event stopped message to tracker
	tkInfo.Uploaded, tkInfo.Downloaded, tkInfo.Left = manager.GetProgress()
	fmt.Println(tkInfo.Uploaded, " ", tkInfo.Downloaded, " ", tkInfo.Left)

	// we calculated tkInfo.Downloaded without accounting for the actual length of
	// the last piece. So, if the total downloaded is some bytes < piece length
	// just say it downloaded the whole thing.
	if tkInfo.Downloaded-iDict.Length < iDict.PieceLength {
		tkInfo.Downloaded = iDict.Length
		tkInfo.Left = 0
	}

	if tkInfo.Left == 0 { // send completed message if the download is complete
		tkInfo.sendGetRequest("completed")
	}
	tkInfo.Disconnect()
}
