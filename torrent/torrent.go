package torrent

import (
	"crypto/sha1"
	"io"
	"os"

	"github.com/zeebo/bencode"
)

type Torrent struct {
	Info         bencode.RawMessage `bencode:"info"`
	Announce     string             `bencode:"announce,omitempty"`
	AnnounceList [][]string         `bencode:"announce-list,omitempty"`
	CreationDate int64              `bencode:"creation date,omitempty"`
	Comment      string             `bencode:"comment,omitempty"`
	CreatedBy    string             `bencode:"created by,omitempty"`
	UrlList      string             `bencode:"url-list,omitempty"`
}

type InfoDict struct {
	Name        string `bencode:"name"`
	Length      int    `bencode:"length"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
}

type TorrentInfoFile struct {
	Name   string   `bencode:"name"`
	Length int      `bencode:"length"`
	Md5Sum string   `bencode:"md5sum"`
	Path   []string `bencode:"path"`
}

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

func (t *Torrent) InfoHash() []byte {

	// peer_id I need & self generate
	// left = length of file downloading (dict)
	//

	hash := sha1.New()
	io.WriteString(hash, string(t.Info))
	infoHash := hash.Sum(nil)

	return infoHash
}
