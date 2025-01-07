package torrentclient

type piece struct {
	pieceIndex int
	data       []byte
}

// TODO: delete this. Added only for test purposes.
func NewPiece(pieceIndex int, data []byte) piece {
	return piece{pieceIndex: pieceIndex, data: data}
}
