package torrentclient

import (
	"context"
	"log/slog"
	"time"

	"github.com/tomsaalex/BitTorrent_Client/customdatatypes"
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
	centralPeerList := make([]peer, 0)

	trackerEventsChannel := make(chan TrackerEvent)
	peerListChannel := make(chan []peer)

	peersToConnectChan := make(chan []peer)
	peerRequestChan := make(chan bool)

	newPieceAcquiredChan := make(chan piece)
	bitfieldOutputChan := make(chan customdatatypes.FixedSizeBitfield)
	bitfieldRequestChan := make(chan bool)

	pieceToWriter := make(chan piece)

	go th.trackerManager(trackerEventsChannel, peerListChannel)
	go th.peerConnectionManager.connectionManager(th.torrentData, &th.torrentStats, th.peerID, peerRequestChan, peersToConnectChan, newPieceAcquiredChan, bitfieldRequestChan, bitfieldOutputChan)
	go fileWriter(pieceToWriter, &th.torrentData)
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
		case newPiece := <-newPieceAcquiredChan:
			slog.LogAttrs(
				context.Background(),
				slog.LevelInfo,
				"Received piece",
				slog.Int("Piece number", newPiece.pieceIndex),
				slog.String("method", "torrentManager"),
			)

			pieceToWriter <- newPiece
		}
	}
}

func NewTorrentHost(torrentData TorrentData, torrentStats TorrentStats, peerID customdatatypes.CustomHash) *torrentHost {
	newTorrent := &torrentHost{torrentData: torrentData, torrentStats: torrentStats, peerID: peerID}

	go newTorrent.torrentManager()

	return newTorrent
}
