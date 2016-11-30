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
var killCh chan bool = make(chan bool) // used to signal kill tracker connection

func sigHandler(ch chan os.Signal, tkInfo TrackerInfo) {
	<-ch
	fmt.Println("Exiting...")
	if err := manager.StopDownload(); err != nil {
		fmt.Println(err)
	}
	killCh <- true // stop tracker connection
	<-killCh       // wait to stop tracker connection
	os.Exit(0)

}

func trackerUpdater(killCh chan bool, tkInfo TrackerInfo, interval int64, iDict InfoDict) {
	// keep announcing to tracker at Interval seconds
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	sentCompleted := false
	go func() {
		fmt.Println("updating...")
		for _ = range ticker.C {
			tkInfo.Uploaded, tkInfo.Downloaded, tkInfo.Left =
				manager.GetProgress()
			tkInfo.sendGetRequest("")
			if sentCompleted { // finished download
				return
			}
		}
	}()

	for {
		toKill := <-killCh
		//	fmt.Println("Tokill: ", toKill, " sentCompleted : ", sentCompleted)

		if sentCompleted == false { // just send completed
			ticker.Stop() // ticker is done
			// Send event stopped message to tracker
			tkInfo.Uploaded, tkInfo.Downloaded, tkInfo.Left = manager.GetProgress()
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
			sentCompleted = true
		}

		if toKill == true { // we are done - return
			tkInfo.Disconnect()
			killCh <- true // finished disconnect
			return
		}
	}
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
	fmt.Println(iDict.Length, iDict.PieceLength)
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
	manager = NewPeerContactManager(&tkInfo, &wg, tInfo, fileName, 10, 10, 10)

	// keep announcing to tracker at Interval seconds
	go trackerUpdater(killCh, tkInfo, interval, iDict)

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

	if err := manager.StartOutgoing(peerList); err != nil {
		fmt.Println(err)
		return
	}

	killCh <- false // don't kill tracker connection but say we completed it
	fmt.Println("Download completed. Waiting for user input...")
	for {

	}
}
