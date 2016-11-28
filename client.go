package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	//	"strings"
	"sync"
	"time"
)

//ClientID is the 20 byte id of our client
//ProtoName is the BitTorrent protocol we are using
const (
	ListenPort = 6881
	ProtoName  = "BitTorrent protocol"
	ClientID   = "DONDESTALABIBLIOTECA"
)

var manager PeerContactManager

func sigHandler(ch chan os.Signal, tkInfo TrackerInfo) {
	<-ch
	fmt.Println("Exiting...")
	if err := manager.StopDownload(); err != nil {
		fmt.Println(err)
	}
	tkInfo.Disconnect()
	os.Exit(0)

}

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
	tkInfo := NewTracker(hash, torrent, &iDict, ListenPort)
	peerList, interval := tkInfo.Connect()
	/*interval := 2
	peerList := make([]Peer, 1, 1)
	peerList[0].IP = "127.0.0.1"
	peerList[0].PeerID = "DONDESTALABIBLIOTECA"
	peerList[0].Port = 6881*/
	//fmt.Printf("%v\n", peerList)

	//Start peer download
	tInfo := TorrentInfo{
		TInfo:        &iDict,
		ClientID:     ClientID,
		ProtoName:    ProtoName,
		ProtoNameLen: len(ProtoName),
		InfoHash:     string(hash[:len(hash)]),
	}
	var wg sync.WaitGroup
	manager = NewPeerContactManager(&wg, tInfo, fileName, 10, 10, 10)

	// keep announcing to tracker at Interval seconds
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	//i := 0
	//j := 10
	//fmt.Println("[", strings.Repeat("#", i), strings.Repeat("-", j), "]")
	go func() {
		for _ = range ticker.C {
			//	fmt.Println("HERE")
			//	j--
			//	i++
			tkInfo.Uploaded, tkInfo.Downloaded, tkInfo.Left =
				manager.GetProgress()
			tkInfo.sendGetRequest("")
			//	fmt.Println("\r[", strings.Repeat("#", i), strings.Repeat("-", j), "]")
		}
	}()

	// Listen for SIGINT to save bitfield to metafile
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt)
	go func() {
		sigHandler(sigChannel, tkInfo)
	}()

	// start listening for requests
	go func() {
		if err := manager.StartIncoming(ListenPort); err != nil {
			fmt.Println("Listen Error\n")
			fmt.Println(err)
			return
		}
	}()

	//	tkInfo.sendGetRequest("")
	if err := manager.StartOutgoing(peerList); err != nil {
		fmt.Println("ERROR!\n")
		return
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
	fmt.Println("Download completed. Waiting for user input...")
	for {

	}
	/*

		tkInfo.Disconnect()

		if err := manager.StopDownload(); err != nil {
			fmt.Println(err)
		}*/
}
