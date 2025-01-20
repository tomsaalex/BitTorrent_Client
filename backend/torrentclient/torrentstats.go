package torrentclient

import (
	"fmt"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type connectionDataStatus struct {
	downloadedData int
	uploadedData   int

	downloadSpeed int
	uploadSpeed   int

	peer         peer
	peerBitfield customdatatypes.FixedSizeBitfield
}

func newConnectionDataStatus(p peer, bitfieldSize int) (connectionDataStatus, error) {
	bitfield, err := customdatatypes.NewFixedSizeBitfield(bitfieldSize)

	if err != nil {
		return connectionDataStatus{}, err
	}

	return connectionDataStatus{peer: p, peerBitfield: *bitfield}, nil
}

type TorrentStats struct {
	torrentInfohash customdatatypes.CustomHash

	uploadedBytes   int
	downloadedBytes int

	pieceIndex customdatatypes.FixedSizeBitfield

	connectionDataStatuses map[string]connectionDataStatus

	piecesStoredToDisk customdatatypes.FixedSizeBitfield
	requestedPieces    []int
	unselectedPieces   []int

	obtainedPiecesChan    chan int
	requestedPiecesChan   chan int
	pieceIndexFullRequest chan bool
	pieceIndexFullReply   chan bool

	unselectedPiecesRequest chan bool
	unselectedPiecesReply   chan []int

	requestedPiecesRequest chan bool
	requestedPiecesReply   chan []int

	storedPiecesAnnouncer chan int

	haveAllPiecesRequest chan bool
	haveAllPiecesReply   chan bool

	statusUpdatesRequest chan bool
	statusUpdatesReply   chan TorrentStatsDTO

	dataReportsChan   chan dataExchangeReport
	speedReportsChan  chan speedExchangeReport
	newPeerPiecesChan chan peerPieceReport
}

func (ts *TorrentStats) StatsKeeper() {
	for {
		select {
		case pieceIndex := <-ts.obtainedPiecesChan:
			ts.markPieceAsObtained(pieceIndex)
		case pieceIndex := <-ts.requestedPiecesChan:
			ts.markPieceAsRequested(pieceIndex)
		case <-ts.pieceIndexFullRequest:
			ts.pieceIndexFullReply <- ts.pieceIndex.IsFull()
		case <-ts.unselectedPiecesRequest:
			// Sending the slice should be fine, since we're not changing the elements of the slice in another thread concurrently,
			// but if I ever want that, this needs to make a copy.
			ts.unselectedPiecesReply <- ts.unselectedPieces
		case <-ts.requestedPiecesRequest:
			// Sending the slice should be fine, since we're not changing the elements of the slice in another thread concurrently,
			// but if I ever want that, this needs to make a copy.
			ts.requestedPiecesReply <- ts.requestedPieces
		case dr := <-ts.dataReportsChan:
			// TODO: Consider making this channel (ts.dataReportsChan) buffered for performance reasons
			connectionStats, found := ts.connectionDataStatuses[dr.remotePeer.fullAddress()]

			if !found {
				connectionStats, _ = newConnectionDataStatus(dr.remotePeer, ts.piecesStoredToDisk.BitCount())
			}

			if dr.trafType == IncomingTraffic {
				connectionStats.downloadedData += dr.exchangedDataCount
				ts.downloadedBytes += dr.exchangedDataCount
			} else {
				connectionStats.uploadedData += dr.exchangedDataCount
				ts.uploadedBytes += dr.exchangedDataCount
			}

			ts.connectionDataStatuses[dr.remotePeer.fullAddress()] = connectionStats
			fmt.Println(ts.downloadedBytes)
		case sr := <-ts.speedReportsChan:
			// TODO: Consider making this channel (ts.speedReportsChan) buffered for performance reasons
			connectionStats, found := ts.connectionDataStatuses[sr.remotePeer.fullAddress()]

			if !found {
				connectionStats, _ = newConnectionDataStatus(sr.remotePeer, ts.piecesStoredToDisk.BitCount())
			}
			if sr.trafType == IncomingTraffic {
				connectionStats.downloadSpeed = sr.connSpeed
			} else {
				connectionStats.uploadSpeed = sr.connSpeed
			}
		case pr := <-ts.newPeerPiecesChan:
			// TODO: Consider making this channel (ts.newPeerPiecesChan) buffered for performance reasons
			connectionStats, found := ts.connectionDataStatuses[pr.remotePeer.fullAddress()]

			if !found {
				connectionStats, _ = newConnectionDataStatus(pr.remotePeer, ts.piecesStoredToDisk.BitCount())
			}

			connectionStats.peerBitfield.SetBit(pr.reportedPieceIndex)

		case <-ts.haveAllPiecesRequest:
			ts.haveAllPiecesReply <- ts.piecesStoredToDisk.IsFull()
		case <-ts.statusUpdatesRequest:
			ts.statusUpdatesReply <- torrentStatsToDTO(ts)
		case pieceIndex := <-ts.storedPiecesAnnouncer:
			ts.piecesStoredToDisk.SetBit(pieceIndex)
		}
	}
}

func (ts *TorrentStats) markPieceAsObtained(pIndex int) {
	ts.pieceIndex.SetBit(pIndex)

	posToRem := -1
	for i := 0; i < len(ts.unselectedPieces); i++ {
		if ts.unselectedPieces[i] == pIndex {
			posToRem = i
		}
	}

	ts.unselectedPieces = append(ts.unselectedPieces[:posToRem], ts.unselectedPieces[posToRem+1:]...)

	for i := len(ts.requestedPieces) - 1; i >= 0; i-- {
		if ts.requestedPieces[i] == pIndex {
			ts.requestedPieces = append(ts.requestedPieces[:i], ts.requestedPieces[i+1:]...)
		}
	}
}

func (ts *TorrentStats) markPieceAsRequested(pIndex int) {
	ts.requestedPieces = append(ts.requestedPieces, pIndex)
}

func (ts *TorrentStats) statusUpdatesChannels() (chan<- bool, <-chan TorrentStatsDTO) {
	return ts.statusUpdatesRequest, ts.statusUpdatesReply
}

func NewTorrentStats(infohash customdatatypes.CustomHash, pieceCount int) (TorrentStats, error) {
	torrentStats := TorrentStats{}

	torrentStats.torrentInfohash = infohash

	bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceCount)
	if err != nil {
		return TorrentStats{}, err
	}
	torrentStats.pieceIndex = *bitfield

	torrentStats.unselectedPieces = bitfield.GetUnsetBitsIndices()
	torrentStats.requestedPieces = make([]int, 0)

	piecesStoredToDisk, err := customdatatypes.NewFixedSizeBitfield(pieceCount)

	if err != nil {
		return TorrentStats{}, err
	}

	torrentStats.piecesStoredToDisk = *piecesStoredToDisk

	torrentStats.connectionDataStatuses = make(map[string]connectionDataStatus)

	torrentStats.obtainedPiecesChan = make(chan int)
	torrentStats.requestedPiecesChan = make(chan int)

	torrentStats.pieceIndexFullRequest = make(chan bool)
	torrentStats.pieceIndexFullReply = make(chan bool)

	torrentStats.unselectedPiecesRequest = make(chan bool)
	torrentStats.unselectedPiecesReply = make(chan []int)

	torrentStats.requestedPiecesRequest = make(chan bool)
	torrentStats.requestedPiecesReply = make(chan []int)

	torrentStats.storedPiecesAnnouncer = make(chan int)

	torrentStats.haveAllPiecesRequest = make(chan bool)
	torrentStats.haveAllPiecesReply = make(chan bool)

	torrentStats.dataReportsChan = make(chan dataExchangeReport)
	torrentStats.speedReportsChan = make(chan speedExchangeReport)
	torrentStats.newPeerPiecesChan = make(chan peerPieceReport)

	torrentStats.statusUpdatesRequest = make(chan bool, 1)
	torrentStats.statusUpdatesReply = make(chan TorrentStatsDTO, 1)

	return torrentStats, nil
}
