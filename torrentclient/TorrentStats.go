package torrentclient

import "github.com/tomsaalex/BitTorrent_Client/customdatatypes"

type TorrentStats struct {
	uploadedBytes   int
	downloadedBytes int

	pieceIndex       customdatatypes.FixedSizeBitfield
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

	ts.unselectedPieces = append(ts.unselectedPieces[:posToRem], ts.unselectedPieces[posToRem+1:]...)
}

func NewTorrentStats(pieceCount int) (TorrentStats, error) {
	bitfield, err := customdatatypes.NewFixedSizeBitfield(pieceCount)
	unselectedPieces := bitfield.GetUnsetBitsIndices()

	if err != nil {
		return TorrentStats{}, err
	}

	return TorrentStats{uploadedBytes: 0, downloadedBytes: 0, pieceIndex: *bitfield, unselectedPieces: unselectedPieces}, nil
}
