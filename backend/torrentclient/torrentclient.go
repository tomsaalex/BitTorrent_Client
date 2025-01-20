package torrentclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const clientID = "TA"
const clientVersion = "0001"

const statusUpdateInterval = 1500 * time.Millisecond

// TorrentClient - A representation of a bittorrent client that can handle multiple torrents at once.
// MUST be initialized using the NewTorrentClient function below
type TorrentClient struct {
	PeerID        customdatatypes.CustomHash
	torrentParser TorrentParser
	torrents      []torrentHost

	AppContext context.Context
}

/*func StartListening() {
	// listen on port 8000
	var listener net.Listener
	var port uint16
	for port = 6881; port <= 6889; port++ {
		listener, listeningError := net.Listen("tcp", ":"+strconv.FormatUint(uint64(port), 10))

		if listeningError == nil {
			break
		}
	}
	fmt.Println("Client has started listening on port " + strconv.FormatUint(uint64(port), 10))

	for {
		peerConnection, _ := listener.Accept()


	}
}*/
/*
func InitiateConnection() {

}*/

func NewTorrentClient() *TorrentClient {
	return &TorrentClient{PeerID: generatePeerID()}
}

func (tc *TorrentClient) ClientRoutine() {
	statusUpdatesTicker := time.NewTicker(statusUpdateInterval)

	for {
		select {
		case <-statusUpdatesTicker.C:
			torrentStatsList := make([]TorrentStatsDTO, 0)

			for _, th := range tc.torrents {
				th.statusUpdateRequest <- true
			}

			for _, th := range tc.torrents {
				torrentStatsList = append(torrentStatsList, <-th.torrentStats.statusUpdatesReply)
			}

			//fmt.Println(torrentStatsList)
			runtime.EventsEmit(tc.AppContext, "torrentStatsUpdated", torrentStatsList)
		}
	}
}

func (tc *TorrentClient) AddTorrent(torrentFilePath string) error {
	newTorrentData, torrentParsingError := tc.torrentParser.ParseTorrentFile(torrentFilePath)

	if torrentParsingError != nil {
		return torrentParsingError
	}
	torrentStats, torrentStatsCreationError := NewTorrentStats(newTorrentData.Infohash, len(newTorrentData.PieceHashes))

	if torrentStatsCreationError != nil {
		return torrentStatsCreationError
	}

	peerConnectionManager := newPeerConnectionManager()

	newTorrentHost := NewTorrentHost(newTorrentData, torrentStats, tc.PeerID, *peerConnectionManager)
	tc.torrents = append(tc.torrents, *newTorrentHost)

	go newTorrentHost.torrentManager()

	return nil
}

func generatePeerID() customdatatypes.CustomHash {
	// Ensure the tag and version are correctly formatted
	stringTag := fmt.Sprintf("-%s%s-", clientID, clientVersion)
	tag := []byte(stringTag)

	// Generate a secure random byte sequence
	randomBytes := make([]byte, 12)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic("Unable to generate random bytes")
	}

	// Process-specific and timestamp-based input
	pid := os.Getpid()
	startTime := time.Now().Unix()

	// Mix process ID and timestamp into the random bytes
	for i := 0; i < len(randomBytes); i++ {
		randomBytes[i] ^= byte((pid + int(startTime)) >> (i % 8))
	}

	croppedRandomBytes := randomBytes[:12] // Ensure total length is 20

	// Combine all parts to form the peer_id
	peerID := append(tag, croppedRandomBytes...)
	return customdatatypes.CustomHash{HashBytes: peerID}
}
