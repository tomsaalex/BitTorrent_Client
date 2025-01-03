package torrentclient

import "github.com/tomsaalex/BitTorrent_Client/customdatatypes"

type TorrentStats struct {
	uploadedBytes   int
	downloadedBytes int

	pieceIndex customdatatypes.FixedSizeBitfield

	requestedPieces  []int
	unselectedPieces []int
}

func (ts *TorrentStats) MarkPieceAsObtained(pIndex int) {
	ts.pieceIndex.SetBit(pIndex)

	posToRem := -1
	for i := 0; i < len(ts.unselectedPieces); i++ {
		if ts.unselectedPieces[i] == pIndex {
			posToRem = i
		}
	}
	if posToRem != -1 {
		// TODO: This is technically impossible, but there's a bug somewhere that causes pieces to be requested multiple times. Remove this check after that is fixed.
		ts.unselectedPieces = append(ts.unselectedPieces[:posToRem], ts.unselectedPieces[posToRem+1:]...)

		for i := len(ts.requestedPieces) - 1; i >= 0; i-- {
			if ts.requestedPieces[i] == pIndex {
				ts.requestedPieces = append(ts.requestedPieces[:i], ts.requestedPieces[i+1:]...)
			}
		}
	}
}

func (ts *TorrentStats) MarkPieceAsRequested(pIndex int) {
	ts.requestedPieces = append(ts.requestedPieces, pIndex)
}

func NewTorrentStats(pieceCount int) (TorrentStats, error) {
	bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceCount)
	unselectedPieces := bitfield.GetUnsetBitsIndices()

	if err != nil {
		return TorrentStats{}, err
	}

	return TorrentStats{uploadedBytes: 0, downloadedBytes: 0, pieceIndex: *bitfield, unselectedPieces: unselectedPieces}, nil
}
