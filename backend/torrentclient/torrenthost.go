package torrentclient

import (
	"context"
	"log/slog"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type TrackerEvent int

const (
	T_STARTED TrackerEvent = iota
	T_STOPPED
	T_COMPLETED
	T_NIL
)

type torrentHost struct {
	torrentData  TorrentData
	torrentStats TorrentStats
	diskManager  DiskManager

	peerID                customdatatypes.CustomHash
	trackConnection       trackerConnection
	peerConnectionManager PeerConnectionManager
}

func (th *torrentHost) trackerManager(eventsChannel <-chan TrackerEvent, peerListChan chan<- []peer) {
	var triggerChannel <-chan time.Time

	trackerInterval := -1

	handleAnnounce := func(eventToExecute TrackerEvent) {
		trackerResponse, err := th.trackConnection.announceRequest(th.torrentData, th.torrentStats, th.peerID, eventToExecute)
		if err != nil {
			panic(err) // TODO replace with proper error handling
		}
		if trackerInterval != trackerResponse.interval {
			trackerInterval = trackerResponse.interval
			triggerChannel = time.After(time.Duration(trackerInterval) * time.Second)
		}

		peerListChan <- trackerResponse.peers
	}

	for {
		select {
		case eventToExecute := <-eventsChannel:
			handleAnnounce(eventToExecute)
		case <-triggerChannel:
			handleAnnounce(T_NIL)
		}
	}
}

func (th *torrentHost) torrentManager() {
	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Started",
		slog.String("method", "torrentManager"),
	)

	piecesStoredToDisk, _ := customdatatypes.NewFixedSizeBitfield(len(th.torrentData.PieceHashes))

	centralPeerList := make([]peer, 0)

	trackerEventsChannel := make(chan TrackerEvent)
	peerListChannel := make(chan []peer)

	peersToConnectChan := make(chan []peer)
	peerRequestChan := make(chan bool)

	bitfieldOutputChan := make(chan customdatatypes.FixedSizeBitfield)
	bitfieldRequestChan := make(chan bool)

	pieceToWriter := make(chan piece)
	newStoredPieceChan := make(chan int)

	torrentDownloadComplete := make(chan bool)

	go th.trackerManager(trackerEventsChannel, peerListChannel)
	go th.torrentStats.StatsKeeper()
	go th.peerConnectionManager.connectionManager(th.torrentData, &th.torrentStats, th.peerID, peerRequestChan, peersToConnectChan, pieceToWriter, bitfieldRequestChan, bitfieldOutputChan, torrentDownloadComplete)
	go th.diskManager.fileWriter(pieceToWriter, newStoredPieceChan, &th.torrentData)
	trackerEventsChannel <- T_STARTED

	for {
		select {
		case centralPeerList = <-peerListChannel:
			// TODO: it's debatable whether this is a good idea. We don't really need both ways to send the peers, I don't think?
			peersToConnectChan <- centralPeerList
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Sent new peers to connection manager",
				slog.String("method", "torrentManager"),
				slog.Int("peerCount", len(centralPeerList)),
			)
		case <-peerRequestChan:
			peersToConnectChan <- centralPeerList
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Sent new peers to connection manager",
				slog.String("method", "torrentManager"),
				slog.Int("peerCount", len(centralPeerList)),
			)
		case <-bitfieldRequestChan:
			bitfieldOutputChan <- th.torrentStats.pieceIndex
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Sent bitfield to connection manager",
				slog.String("method", "torrentManager"),
			)
		case pieceIndex := <-newStoredPieceChan:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Received piece",
				slog.Int("Piece number", pieceIndex),
				slog.String("method", "torrentManager"),
			)

			piecesStoredToDisk.SetBit(pieceIndex)

			if piecesStoredToDisk.IsFull() {
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"FILE DOWNLOAD COMPLETE",
					slog.String("method", "torrentManager"),
				)
			}
		}
	}
}

func NewTorrentHost(torrentData TorrentData, torrentStats TorrentStats, peerID customdatatypes.CustomHash, pcm PeerConnectionManager) *torrentHost {

	newTorrent := &torrentHost{torrentData: torrentData, torrentStats: torrentStats, peerID: peerID, peerConnectionManager: pcm}
	return newTorrent
}
