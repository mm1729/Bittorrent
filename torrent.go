package main

import (
	"crypto/sha1"
	"io"
	"log"
	"os"

	"github.com/zeebo/bencode"
)

// Torrent is the struct containing the decoded torrent meta file
type Torrent struct {
	Info         bencode.RawMessage `bencode:"info"`
	Announce     string             `bencode:"announce,omitempty"`
	AnnounceList [][]string         `bencode:"announce-list,omitempty"`
	CreationDate int64              `bencode:"creation date,omitempty"`
	Comment      string             `bencode:"comment,omitempty"`
	CreatedBy    string             `bencode:"created by,omitempty"`
	URLList      string             `bencode:"url-list,omitempty"`
}

// InfoDict is the info dictionary
type InfoDict struct {
	Name        string `bencode:"name"`
	Length      int    `bencode:"length"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
}

//InfoFile is currently unused
type InfoFile struct {
	Name   string   `bencode:"name"`
	Length int      `bencode:"length"`
	Md5Sum string   `bencode:"md5sum"`
	Path   []string `bencode:"path"`
}

// NewTorrent creates a new torrent struct from the file at torrentPath
func NewTorrent(torrentPath string) (torrent *Torrent, err error) {
	var file *os.File

	file, err = os.Open(torrentPath)
	if err != nil {
		return
	}

	err = bencode.NewDecoder(file).Decode(&torrent)
	if err != nil {
		panic(err)
	}

	return
}

//InfoHash returns the hash of the bencoded info dictionary
func (t *Torrent) InfoHash() []byte {

	// peer_id I need & self generate
	// left = length of file downloading (dict)
	//

	hash := sha1.New()
	io.WriteString(hash, string(t.Info))
	infoHash := hash.Sum(nil)

	return infoHash
}

//InfoDict returns the decoded info dictionary in the torrent info
func (t *Torrent) InfoDict() (id InfoDict) {
	if err := bencode.DecodeBytes(t.Info, &id); err != nil {
		log.Fatal("Unable to parse the Info Dictionary in the torrent file", err)
	}

	return id
}
