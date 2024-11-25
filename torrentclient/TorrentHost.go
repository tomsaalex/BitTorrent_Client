package torrentclient

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	Bencoding "github.com/tomsaalex/BitTorrent_Client/bencoding"
)

type TrackerEvent int

const (
	T_STARTED TrackerEvent = iota
	T_STOPPED
	T_COMPLETED
	T_NIL
)

type torrentHost struct {
	torrentData  Bencoding.TorrentData
	torrentStats TorrentStats

	peerID          []byte
	trackConnection TrackerConnection
}

func (th *torrentHost) MakeRequestToTracker() {
	trackerResponse, err := th.trackConnection.announceRequest(th.torrentData, th.torrentStats, th.peerID, T_STARTED)

	if err != nil {
		panic("This really shouldn't happen")
	}

	testPeer := trackerResponse.peers[1]

	pstr := "BitTorrent protocol"
	var handshakeBuffer bytes.Buffer

	reservedBytes := make([]byte, 8)
	handshakeBuffer.WriteByte(byte(19))
	handshakeBuffer.WriteString(pstr)
	handshakeBuffer.Write(reservedBytes)
	handshakeBuffer.Write(th.torrentData.Infohash.HashBytes)
	handshakeBuffer.Write(th.peerID)

	encodedHandshake := handshakeBuffer.Bytes()
	connectAddress := testPeer.ip + ":" + strconv.Itoa(int(testPeer.port))

	response := make([]byte, 100)
	peerConn, err := net.Dial("tcp", connectAddress)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer peerConn.Close()

	peerConn.Write(encodedHandshake)

	numBRead, readErr := peerConn.Read(response)

	if readErr != nil {
		panic("This really shouldn't have happened")
	}

	fmt.Printf("Read %d bytes", numBRead)
	fmt.Printf(string(response))
}

func NewTorrentHost(torrentData Bencoding.TorrentData, torrentStats TorrentStats, peerID []byte) *torrentHost {
	return &torrentHost{torrentData: torrentData, torrentStats: torrentStats, peerID: peerID}
}
