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

	statusUpdateRequest chan<- bool
	statusUpdateReply   <-chan TorrentStatsDTO

	torrentUpdateRequest chan bool
	torrentUpdateReply   chan TorrentDTO

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

	bitfieldOutputChan := make(chan customdatatypes.FixedSizeBitfield)
	bitfieldRequestChan := make(chan bool)

	pieceToWriter := make(chan piece)
	newStoredPieceChan := make(chan int)

	havePieceAnnouncer := make(chan int)

	go th.trackerManager(trackerEventsChannel, peerListChannel)
	go th.torrentStats.StatsKeeper()
	go th.peerConnectionManager.connectionManager(th.torrentData, &th.torrentStats, th.peerID, peerRequestChan, peersToConnectChan, pieceToWriter, havePieceAnnouncer)
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

			th.torrentStats.storedPiecesAnnouncer <- pieceIndex
			havePieceAnnouncer <- pieceIndex

			th.torrentStats.haveAllPiecesRequest <- true
			haveAllPieces := <-th.torrentStats.haveAllPiecesReply
			if haveAllPieces {
				trackerEventsChannel <- T_COMPLETED
				slog.LogAttrs(
					context.Background(),
					slog.LevelInfo,
					"FILE DOWNLOAD COMPLETE",
					slog.String("method", "torrentManager"),
				)
			}
		case <-th.torrentUpdateRequest:
			th.statusUpdateRequest <- true
			torrentStatsDTO := <-th.statusUpdateReply
			torrentDTO := TorrentDTO{}
			torrentDTO.TorrentData = torrentDataToDTO(&th.torrentData)
			torrentDTO.TorrentStats = torrentStatsDTO

			th.torrentUpdateReply <- torrentDTO
		}
	}
}

func NewTorrentHost(torrentData TorrentData, torrentStats TorrentStats, peerID customdatatypes.CustomHash, pcm PeerConnectionManager) *torrentHost {
	statsReqChan, statsReplyChan := torrentStats.statusUpdatesChannels()

	torrReqChan := make(chan bool)
	torrReplyChan := make(chan TorrentDTO)

	newTorrent := &torrentHost{torrentData: torrentData, torrentStats: torrentStats, peerID: peerID, peerConnectionManager: pcm, statusUpdateRequest: statsReqChan, statusUpdateReply: statsReplyChan, torrentUpdateRequest: torrReqChan, torrentUpdateReply: torrReplyChan}
	return newTorrent
}
