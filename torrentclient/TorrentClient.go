package torrentclient

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	Bencoding "github.com/tomsaalex/BitTorrent_Client/bencoding"
)

const ClientId = "TA"
const ClientVersion = "0001"

// MUST be initialized using the NewTorrentClient function below
type TorrentClient struct {
	PeerId        []byte
	torrentParser Bencoding.TorrentParser
	torrents      []torrentHost
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
	return &TorrentClient{PeerId: generatePeerID()}
}

func (tc *TorrentClient) AddTorrent(torrentFilePath string) {
	newTorrentData, torrentParsingError := tc.torrentParser.ParseTorrentFile(torrentFilePath)

	if torrentParsingError != nil {
		panic(torrentParsingError)
	}

	newTorrentHost := NewTorrentHost(newTorrentData, TorrentStats{}, tc.PeerId)
	tc.torrents = append(tc.torrents, *newTorrentHost)

	newTorrentHost.MakeRequestToTracker()
}

func generatePeerID() []byte {
	// Ensure the tag and version are correctly formatted
	stringTag := fmt.Sprintf("-%s%s-", ClientId, ClientVersion)
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
	return append(tag, croppedRandomBytes...)
}
