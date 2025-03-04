package torrentclient

import (
	"context"
	"fmt"
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
	torrentStats *TorrentStats
	diskManager  DiskManager

	torrentUpdateRequest chan bool
	torrentUpdateReply   chan TorrentDTO

	peerID                customdatatypes.CustomHash
	trackConnection       trackerConnection
	peerConnectionManager PeerConnectionManager
}

func (th *torrentHost) trackerManager(ctx context.Context, eventsChannel <-chan TrackerEvent, peerListChan chan<- []peer) {
	var triggerChannel <-chan time.Time

	trackerInterval := -1

	handleAnnounce := func(eventToExecute TrackerEvent) {
		trackerResponse, err := th.trackConnection.announceRequest(ctx, th.torrentData, th.torrentStats, th.peerID, eventToExecute)
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
		case <-ctx.Done():
			fmt.Println("Exitted out of trackerManager")
			return
		}
	}
}

func (th *torrentHost) recheckTorrent() error {
	res := make(chan pieceValidationResult)
	prevTorrentState := th.torrentStats.getState()
	defer func() {
		th.torrentStats.changeState(prevTorrentState)
	}()

	th.torrentStats.changeState(Rechecking)

	validPieces, err := customdatatypes.NewFixedSizeBitfield(len(th.torrentData.PieceHashes))
	if err != nil {
		return err
	}

	go th.diskManager.checkTorrentIntegrity(th.torrentData, res)

	for validationResult := range res {
		if validationResult.valid {
			validPieces.SetBit(validationResult.pieceIndex)
		}
		th.torrentStats.incrementRecheckedPiecesCount()
	}

	th.torrentStats.UpdateRecheckedPieces(validPieces)
	return nil
}

func (th *torrentHost) torrentManager(ctx context.Context) {
	slog.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"Started",
		slog.String("method", "torrentManager"),
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	centralPeerList := make([]peer, 0)

	trackerEventsChannel := make(chan TrackerEvent)
	peerListChannel := make(chan []peer)

	peersToConnectChan := make(chan []peer)
	peerRequestChan := make(chan bool)

	bitfieldOutputChan := make(chan *customdatatypes.FixedSizeBitfield)
	bitfieldRequestChan := make(chan bool)

	pieceToWriter := make(chan piece)
	newStoredPieceChan := make(chan int)

	havePieceAnnouncer := make(chan int)

	blockRequestsInput := make(chan BlockRetrievalRequest)

	//th.recheckTorrent()

	go th.trackerManager(ctx, trackerEventsChannel, peerListChannel)
	go th.peerConnectionManager.connectionManager(ctx, th.torrentData, th.torrentStats, th.peerID, peerRequestChan, peersToConnectChan, pieceToWriter, havePieceAnnouncer, blockRequestsInput)
	go th.diskManager.fileWriter(ctx, pieceToWriter, newStoredPieceChan, blockRequestsInput, &th.torrentData)
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

			th.torrentStats.piecesStoredToDisk.SetBit(pieceIndex)
			havePieceAnnouncer <- pieceIndex

			haveAllPieces := th.torrentStats.piecesStoredToDisk.IsFull()
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
			torrentStatsDTO := torrentStatsSnapshot(th.torrentStats)
			torrentDTO := TorrentDTO{}
			torrentDTO.TorrentData = torrentDataToDTO(&th.torrentData)
			torrentDTO.TorrentStats = torrentStatsDTO

			th.torrentUpdateReply <- torrentDTO
		case <-ctx.Done():
			return
		}
	}
}

func NewTorrentHost(torrentData TorrentData, torrentStats *TorrentStats, peerID customdatatypes.CustomHash, pcm PeerConnectionManager) *torrentHost {
	torrReqChan := make(chan bool)
	torrReplyChan := make(chan TorrentDTO)

	newTorrent := &torrentHost{torrentData: torrentData, torrentStats: torrentStats, peerID: peerID, peerConnectionManager: pcm, torrentUpdateRequest: torrReqChan, torrentUpdateReply: torrReplyChan}
	return newTorrent
}
