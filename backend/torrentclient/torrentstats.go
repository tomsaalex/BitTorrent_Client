package torrentclient

import (
	"fmt"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type connectionDataStatus struct {
	downloadedData int
	uploadedData   int
	peer           peer
}

type TorrentStats struct {
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

	dataReportsChan chan dataExchangeReport
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
			connectionStats, found := ts.connectionDataStatuses[dr.remotePeer.fullAddress()]

			if !found {
				connectionStats = connectionDataStatus{peer: dr.remotePeer}
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
		case <-ts.haveAllPiecesRequest:
			ts.haveAllPiecesReply <- ts.piecesStoredToDisk.IsFull()
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

func NewTorrentStats(pieceCount int) (TorrentStats, error) {
	bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceCount)
	unselectedPieces := bitfield.GetUnsetBitsIndices()
	requestedPieces := make([]int, 0)

	if err != nil {
		return TorrentStats{}, err
	}

	piecesStoredToDisk, err := customdatatypes.NewFixedSizeBitfield(pieceCount)

	if err != nil {
		return TorrentStats{}, err
	}

	connectionDataStatuses := make(map[string]connectionDataStatus)

	obtainedPiecesChan := make(chan int)
	requestedPiecesChan := make(chan int)

	pieceIndexFullRequest := make(chan bool)
	pieceIndexFullReply := make(chan bool)

	unselectedPiecesRequest := make(chan bool)
	unselectedPiecesReply := make(chan []int)

	requestedPiecesRequest := make(chan bool)
	requestedPiecesReply := make(chan []int)

	dataReportsChan := make(chan dataExchangeReport)

	storedPiecesAnnouncer := make(chan int)

	haveAllPiecesRequest := make(chan bool)
	haveAllPiecesReply := make(chan bool)

	return TorrentStats{uploadedBytes: 0, downloadedBytes: 0, pieceIndex: *bitfield, unselectedPieces: unselectedPieces, requestedPieces: requestedPieces, obtainedPiecesChan: obtainedPiecesChan, requestedPiecesChan: requestedPiecesChan, pieceIndexFullRequest: pieceIndexFullRequest, pieceIndexFullReply: pieceIndexFullReply, unselectedPiecesRequest: unselectedPiecesRequest, unselectedPiecesReply: unselectedPiecesReply, requestedPiecesRequest: requestedPiecesRequest, requestedPiecesReply: requestedPiecesReply, dataReportsChan: dataReportsChan, connectionDataStatuses: connectionDataStatuses, piecesStoredToDisk: *piecesStoredToDisk, storedPiecesAnnouncer: storedPiecesAnnouncer, haveAllPiecesRequest: haveAllPiecesRequest, haveAllPiecesReply: haveAllPiecesReply}, nil
}
