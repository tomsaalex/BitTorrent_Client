package torrentclient

type BlockRequest struct {
	remotePeer  peer
	pieceIndex  int
	blockStart  int
	blockLength int
}

func (br *BlockRequest) equal(other *BlockRequest) bool {
	return br.remotePeer.ip == other.remotePeer.ip && br.remotePeer.port == other.remotePeer.port && br.pieceIndex == other.pieceIndex &&
		br.blockStart == other.blockStart && br.blockLength == other.blockLength
}
