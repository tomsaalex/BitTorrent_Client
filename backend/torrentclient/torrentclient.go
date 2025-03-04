package torrentclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

const clientID = "TA"
const clientVersion = "0001"

// TorrentClient - A representation of a bittorrent client that can handle multiple torrents at once.
// MUST be initialized using the NewTorrentClient function below
type TorrentClient struct {
	PeerID        customdatatypes.CustomHash
	torrentParser TorrentParser

	AppContext context.Context
	Cancel     context.CancelFunc

	addTorrentChan chan string

	reportRequestChan chan bool
	reportReplyChan   chan AggregateReport

	servedTorrentRequest chan []byte
	servedTorrentReply   chan (chan<- connBootstrapInfo)
}

type AggregateReport struct {
	Torrents []TorrentDTO `json:"torrents"`
}

func NewTorrentClient() *TorrentClient {
	tc := &TorrentClient{PeerID: generatePeerID()}

	tc.addTorrentChan = make(chan string)

	tc.reportRequestChan = make(chan bool)
	tc.reportReplyChan = make(chan AggregateReport)

	tc.servedTorrentRequest = make(chan []byte)
	tc.servedTorrentReply = make(chan (chan<- connBootstrapInfo))

	return tc
}

func (tc *TorrentClient) LaunchRoutines() {
	go tc.clientRoutine()
}

func (tc *TorrentClient) clientRoutine() {
	torrents := make([]torrentHost, 0)

	ctx, cancel := context.WithCancel(tc.AppContext)
	defer cancel()
	tc.Cancel = cancel

	go handleIncomingConnections(ctx, tc.servedTorrentRequest, tc.servedTorrentReply)

	for {
		select {
		case torrentFilePath := <-tc.addTorrentChan:
			newTorrent, err := tc.handleAddTorrent(torrentFilePath)
			if err != nil {
				// TODO: Better error handling, ideally on the frontend
				log.Fatal(err)
				continue
			}

			torrents = append(torrents, *newTorrent)

			go newTorrent.torrentManager(ctx)

		case <-tc.reportRequestChan:
			tc.reportReplyChan <- tc.handleGenerateAggregateReport(torrents)
		case infohash := <-tc.servedTorrentRequest:
			foundTorrent := false
			for _, th := range torrents {
				if bytes.Equal(th.torrentData.Infohash.HashBytes, infohash) {
					tc.servedTorrentReply <- th.peerConnectionManager.connectionIntegration
					foundTorrent = true
					break
				}
			}
			if !foundTorrent {
				tc.servedTorrentReply <- nil
			}
		case <-ctx.Done():
			fmt.Println("Exited clientRoutine")
			return
		}
	}
}

func (tc *TorrentClient) handleGenerateAggregateReport(torrents []torrentHost) AggregateReport {
	torrentStatsList := make([]TorrentDTO, 0)

	for _, th := range torrents {
		th.torrentUpdateRequest <- true
	}

	for _, th := range torrents {
		torrentDTO := <-th.torrentUpdateReply
		torrentStatsList = append(torrentStatsList, torrentDTO)
	}

	aggregateReport := AggregateReport{Torrents: torrentStatsList}
	return aggregateReport
}

func (tc *TorrentClient) GenerateAggregateReport() AggregateReport {
	tc.reportRequestChan <- true
	report := <-tc.reportReplyChan
	return report
}

func (tc *TorrentClient) handleAddTorrent(torrentFilePath string) (*torrentHost, error) {
	newTorrentData, torrentParsingError := tc.torrentParser.ParseTorrentFile(torrentFilePath)

	if torrentParsingError != nil {
		return nil, torrentParsingError
	}
	torrentStats, torrentStatsCreationError := NewTorrentStats(newTorrentData.Infohash, len(newTorrentData.PieceHashes))

	if torrentStatsCreationError != nil {
		return nil, torrentStatsCreationError
	}

	peerConnectionManager := newPeerConnectionManager()

	return NewTorrentHost(newTorrentData, torrentStats, tc.PeerID, *peerConnectionManager), nil
}

func (tc *TorrentClient) AddTorrent(torrentFilePath string) {
	// TODO: Error handling in the frontend... Eventually
	tc.addTorrentChan <- torrentFilePath
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
