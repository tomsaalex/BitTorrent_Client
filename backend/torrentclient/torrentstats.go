package torrentclient

import (
	"sync"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type torrentState int

const (
	Running torrentState = iota
	Paused
	Rechecking
)

type connectionDataStatus struct {
	downloadedData int
	uploadedData   int

	downloadSpeed int
	uploadSpeed   int

	peer         peer
	peerBitfield *customdatatypes.FixedSizeBitfield
}

func newConnectionDataStatus(p peer, bitfieldSize int) (connectionDataStatus, error) {
	bitfield, err := customdatatypes.NewFixedSizeBitfield(bitfieldSize)

	if err != nil {
		return connectionDataStatus{}, err
	}

	return connectionDataStatus{peer: p, peerBitfield: bitfield}, nil
}

type TorrentStats struct {
	torrentInfohash customdatatypes.CustomHash

	state   torrentState
	stateMu sync.Mutex

	recheckedPiecesCount   int
	recheckedPiecesCountMu sync.Mutex

	uploadedBytes   int
	uploadedBytesMu sync.Mutex

	downloadedBytes   int
	downloadedBytesMu sync.Mutex

	partialPiecesBytes   int
	partialPiecesBytesMu sync.Mutex

	pieceIndex *customdatatypes.FixedSizeBitfield

	connectionDataStatuses   map[string]connectionDataStatus
	connectionDataStatusesMu sync.Mutex

	piecesStoredToDisk *customdatatypes.FixedSizeBitfield
	requestedBlocks    []BlockRequest
	requestedBlocksMu  sync.Mutex

	requestedPieces   []int
	requestedPiecesMu sync.Mutex

	unselectedPieces   []int
	unselectedPiecesMu sync.Mutex
}

func (ts *TorrentStats) unselectedPiecesCopy() []int {
	// TODO: Consider replacing append with copy (must initialize dest with the proper length) for performance.
	ts.unselectedPiecesMu.Lock()
	defer ts.unselectedPiecesMu.Unlock()
	return append([]int{}, ts.unselectedPieces...)
}

func (ts *TorrentStats) requestedPiecesCopy() []int {
	// TODO: Consider replacing append with copy (must initialize dest with the proper length) for performance.
	ts.requestedPiecesMu.Lock()
	defer ts.requestedPiecesMu.Unlock()
	return append([]int{}, ts.requestedPieces...)
}

func (ts *TorrentStats) requestedBlocksCopy() []BlockRequest {
	// TODO: Consider replacing append with copy (must initialize dest with the proper length) for performance.
	ts.requestedBlocksMu.Lock()
	defer ts.requestedBlocksMu.Unlock()
	return append([]BlockRequest{}, ts.requestedBlocks...)
}

func (ts *TorrentStats) updateRemotePeerPieces(report peerPieceReport) {
	ts.connectionDataStatusesMu.Lock()
	defer ts.connectionDataStatusesMu.Unlock()

	connectionStats, found := ts.connectionDataStatuses[report.remotePeer.fullAddress()]

	if !found {
		connectionStats, _ = newConnectionDataStatus(report.remotePeer, ts.piecesStoredToDisk.BitCount())
	}

	connectionStats.peerBitfield.SetBit(report.reportedPieceIndex)
}

func (ts *TorrentStats) addSpeedReport(report speedExchangeReport) {
	ts.connectionDataStatusesMu.Lock()
	defer ts.connectionDataStatusesMu.Unlock()
	connectionStats, found := ts.connectionDataStatuses[report.remotePeer.fullAddress()]

	if !found {
		connectionStats, _ = newConnectionDataStatus(report.remotePeer, ts.piecesStoredToDisk.BitCount())
	}
	if report.trafType == IncomingTraffic {
		connectionStats.downloadSpeed = report.connSpeed
	} else {
		connectionStats.uploadSpeed = report.connSpeed
	}

	ts.connectionDataStatuses[report.remotePeer.fullAddress()] = connectionStats
}

func (ts *TorrentStats) addDataReport(report dataExchangeReport) {
	ts.connectionDataStatusesMu.Lock()
	defer ts.connectionDataStatusesMu.Unlock()

	connectionStats, found := ts.connectionDataStatuses[report.remotePeer.fullAddress()]

	if !found {
		connectionStats, _ = newConnectionDataStatus(report.remotePeer, ts.piecesStoredToDisk.BitCount())
	}

	if report.trafType == IncomingTraffic {
		connectionStats.downloadedData += report.exchangedDataCount

		ts.downloadedBytesMu.Lock()
		defer ts.downloadedBytesMu.Unlock()
		ts.downloadedBytes += report.exchangedDataCount
	} else {
		connectionStats.uploadedData += report.exchangedDataCount

		ts.uploadedBytesMu.Lock()
		defer ts.uploadedBytesMu.Unlock()
		ts.uploadedBytes += report.exchangedDataCount
	}

	ts.connectionDataStatuses[report.remotePeer.fullAddress()] = connectionStats
}

func (ts *TorrentStats) removePieceFromRequested(pIndex int) {
	ts.requestedPiecesMu.Lock()
	defer ts.requestedPiecesMu.Unlock()

	for i := len(ts.requestedPieces) - 1; i >= 0; i-- {
		if ts.requestedPieces[i] == pIndex {
			ts.requestedPieces = append(ts.requestedPieces[:i], ts.requestedPieces[i+1:]...)
		}
	}
}

func (ts *TorrentStats) markPieceAsObtained(pIndex int) {
	ts.pieceIndex.SetBit(pIndex)
	ts.removePieceFromRequested(pIndex)
}

func (ts *TorrentStats) markPieceAsRequested(pIndex int) {
	ts.requestedPiecesMu.Lock()
	ts.requestedPieces = append(ts.requestedPieces, pIndex)
	ts.requestedPiecesMu.Unlock()

	ts.unselectedPiecesMu.Lock()
	defer ts.unselectedPiecesMu.Unlock()
	posToRem := -1
	for i, unselectedPiece := range ts.unselectedPieces {
		if unselectedPiece == pIndex {
			posToRem = i
		}
	}

	ts.unselectedPieces = append(ts.unselectedPieces[:posToRem], ts.unselectedPieces[posToRem+1:]...)
}

func (ts *TorrentStats) markBlockAsRequested(block BlockRequest) {
	ts.requestedBlocksMu.Lock()
	defer ts.requestedBlocksMu.Unlock()

	ts.requestedBlocks = append(ts.requestedBlocks, block)
}

func (ts *TorrentStats) markBlockAsObtained(block BlockRequest) {
	ts.requestedBlocksMu.Lock()
	defer ts.requestedBlocksMu.Unlock()
	posToRem := -1
	for i, blockReq := range ts.requestedBlocks {
		if blockReq.pieceIndex == block.pieceIndex && blockReq.blockStart == block.blockStart {
			posToRem = i
			break
		}
	}
	// TODO: Ideally there shouldn't be any situation where posToRem is negative or out of bounds, but maybe some check or assertion would be good
	ts.requestedBlocks = append(ts.requestedBlocks[:posToRem], ts.requestedBlocks[posToRem+1:]...)
}

func (ts *TorrentStats) cancelRequestsToPeer(peerConn *peerConnection) {
	ts.requestedBlocksMu.Lock()
	defer ts.requestedBlocksMu.Unlock()
	validRequests := make([]BlockRequest, 0)
	for _, req := range ts.requestedBlocks {
		if req.remotePeer.equal(&peerConn.otherPeer) {
			index := req.pieceIndex
			begin := req.blockStart
			length := req.blockLength
			peerConn.input <- cancelMessage{index: index, begin: begin, length: length}
		} else {
			validRequests = append(validRequests, req)
		}
	}

	ts.requestedBlocks = validRequests
}

func (ts *TorrentStats) getState() torrentState {
	ts.stateMu.Lock()
	defer ts.stateMu.Unlock()

	return ts.state
}

func (ts *TorrentStats) changeState(newState torrentState) {
	ts.stateMu.Lock()
	defer ts.stateMu.Unlock()

	switch newState {
	case Rechecking:
		ts.recheckedPiecesCountMu.Lock()
		defer ts.recheckedPiecesCountMu.Unlock()

		ts.recheckedPiecesCount = 0
		ts.state = newState
	case Running:
		ts.state = newState
	}
}

func (ts *TorrentStats) incrementRecheckedPiecesCount() {
	ts.recheckedPiecesCountMu.Lock()
	defer ts.recheckedPiecesCountMu.Unlock()

	ts.recheckedPiecesCount++
}

func (ts *TorrentStats) UpdateRecheckedPieces(pieceIndex *customdatatypes.FixedSizeBitfield) error {
	err := ts.pieceIndex.ImportBitfield(pieceIndex.ExposeBitfield())
	if err != nil {
		return err
	}

	err = ts.piecesStoredToDisk.ImportBitfield(pieceIndex.ExposeBitfield())
	if err != nil {
		return err
	}

	ts.unselectedPiecesMu.Lock()
	defer ts.unselectedPiecesMu.Unlock()

	ts.unselectedPieces = pieceIndex.GetUnsetBitsIndices()
	return nil
}

func (ts *TorrentStats) addPartialPiecesBytes(newlyAddedBytes int) {
	ts.partialPiecesBytesMu.Lock()
	defer ts.partialPiecesBytesMu.Unlock()

	ts.partialPiecesBytes += newlyAddedBytes
}

func (ts *TorrentStats) getPartialPiecesBytes() int {
	ts.partialPiecesBytesMu.Lock()
	defer ts.partialPiecesBytesMu.Unlock()

	return ts.partialPiecesBytes
}

func NewTorrentStats(infohash customdatatypes.CustomHash, pieceCount int) (*TorrentStats, error) {
	torrentStats := TorrentStats{}

	torrentStats.state = Running
	torrentStats.torrentInfohash = infohash

	bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceCount)
	if err != nil {
		return nil, err
	}
	torrentStats.pieceIndex = bitfield

	torrentStats.unselectedPieces = bitfield.GetUnsetBitsIndices()
	torrentStats.requestedPieces = make([]int, 0)

	piecesStoredToDisk, err := customdatatypes.NewFixedSizeBitfield(pieceCount)

	if err != nil {
		return nil, err
	}

	torrentStats.piecesStoredToDisk = piecesStoredToDisk

	torrentStats.connectionDataStatuses = make(map[string]connectionDataStatus)

	return &torrentStats, nil
}
