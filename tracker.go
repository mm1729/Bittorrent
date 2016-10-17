package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeebo/bencode"
)

//TrackerInfo contains the infomation needed to connect and disconnect from tracker
type TrackerInfo struct {
	URL string //url of tracker to send the GET request
}

//TrackerResponse is the decoded response of the Tracker
type TrackerResponse struct {
	Complete    int64
	Downloaded  int64
	Incomplete  int64
	MinInterval int64
	Peers       []Peer
}

// Peer is the struct containing the ip, peerid and the port of a peer
type Peer struct {
	IP     string `bencode:"ip"`
	PeerID string `bencode:"peer id"`
	Port   int64  `bencode:"port"`
}

func getURL(hash []byte, tInfo *Torrent, iDict *InfoDict) string {
	hexStr := []rune(hex.EncodeToString(hash))
	urlHash := ""

	for i := 0; i < len(hexStr); i += 2 {
		urlHash += "%" + string(hexStr[i]) + string(hexStr[i+1])
	}

	url := tInfo.Announce + "?info_hash=" + urlHash + "&peer_id=DONDESTALABIBLIOTECA&port=6881&uploaded=0&downloaded=0&left=" + strconv.Itoa(iDict.Length)

	return url
}

//NewTracker initializes a new tracker and takes a byte array of the info hash
func NewTracker(hash []byte, tInfo *Torrent, iDict *InfoDict) (trkInfo TrackerInfo) {
	trkInfo.URL = getURL(hash, tInfo, iDict)
	return
}

func (trkInfo TrackerInfo) sendGetRequest(event string) []byte {
	fmt.Printf("\nSending GET Request to : %s\n", trkInfo.URL+"&event="+event)
	resp, err := http.Get(trkInfo.URL)
	defer resp.Body.Close()
	if err != nil {
		log.Fatal("Unable to contact Tracker", err)
	}

	fmt.Printf("\nResponse received from Tracker with status code: %d\n", resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Internal error reading the response from tracker", err)
	}

	return body
}

//Connect sends a GET request to the tracker
func (trkInfo TrackerInfo) Connect() (peerList []Peer) {
	body := trkInfo.sendGetRequest("started")
	var dec TrackerResponse
	errDecode := bencode.DecodeBytes(body, &dec)
	if errDecode != nil {
		log.Fatal("Unable to decode the Tracker Respose\n", errDecode)
	}

	for _, p := range dec.Peers {
		if strings.HasPrefix(p.PeerID, "-RU") {
			peerList = append(peerList, p)
		}
	}
	return
}

// Disconnect sends a event stopped status to the tracker
func (trkInfo TrackerInfo) Disconnect() {
	trkInfo.sendGetRequest("stopped")
}
