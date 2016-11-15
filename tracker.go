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
	urlStub    string // this is the part that is always constant
	Uploaded   int
	Downloaded int
	Left       int
}

//TrackerResponse is the decoded response of the Tracker
type TrackerResponse struct {
	Complete    int64
	Downloaded  int64
	Incomplete  int64
	Interval    int64
	MinInterval int64 `bencode:"min interval"`
	Peers       []Peer
}

// Peer is the struct containing the ip, peerid and the port of a peer
type Peer struct {
	IP     string `bencode:"ip"`
	PeerID string `bencode:"peer id"`
	Port   int64  `bencode:"port"`
}

//NewTracker initializes a new tracker CONNECTION and takes a byte array of the info hash
func NewTracker(hash []byte, tInfo *Torrent, iDict *InfoDict, port int) (trkInfo TrackerInfo) {
	hexStr := []rune(hex.EncodeToString(hash))
	urlHash := ""

	for i := 0; i < len(hexStr); i += 2 {
		urlHash += "%" + string(hexStr[i]) + string(hexStr[i+1])
	}

	trkInfo.urlStub = tInfo.Announce + "?info_hash=" + urlHash + "&peer_id=DONDESTALABIBLIOTECA&port=" + strconv.Itoa(port)
	trkInfo.Uploaded, trkInfo.Downloaded, trkInfo.Left = 0, 0, iDict.Length
	return
}

func (trkInfo TrackerInfo) sendGetRequest(event string) []byte {
	url := trkInfo.urlStub + "&uploaded=" + strconv.Itoa(trkInfo.Uploaded) + "&downloaded=" +
		strconv.Itoa(trkInfo.Downloaded) + "&left=" + strconv.Itoa(trkInfo.Left)

	if event != "" { // add event if it s a sepcial event like started or completed
		url += "&event=" + event
	}

	fmt.Printf("\nSending GET Request to : %s\n", url)
	resp, err := http.Get(url)
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
func (trkInfo TrackerInfo) Connect() ([]Peer, int64) {
	body := trkInfo.sendGetRequest("started")
	var dec TrackerResponse
	errDecode := bencode.DecodeBytes(body, &dec)
	if errDecode != nil {
		log.Fatal("Unable to decode the Tracker Respose\n", errDecode)
	}

	var peerList []Peer
	for _, p := range dec.Peers {
		if strings.HasPrefix(p.PeerID, "-RU11") {
			peerList = append(peerList, p)
		}
	}

	return peerList, dec.Interval
}

// Disconnect sends a event stopped status to the tracker
func (trkInfo TrackerInfo) Disconnect() {
	trkInfo.sendGetRequest("stopped")
}
