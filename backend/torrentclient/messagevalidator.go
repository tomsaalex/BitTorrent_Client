package torrentclient

import "strconv"

func validateRequestMessage(reqMsg requestMessage, tData *TorrentData) error {
	pIndex := reqMsg.index
	blockBegin := reqMsg.begin
	blockLen := reqMsg.length

	if pIndex < 0 || pIndex >= len(tData.PieceHashes) {
		return &InvalidMessageError{Message: "Piece index out of bounds : " + strconv.Itoa(pIndex), MsgType: "REQUEST_MESSAGE"}
	}

	pieceLength := tData.PieceLength

	remainder := tData.TorrentSize % tData.PieceLength
	if pIndex == len(tData.PieceHashes)-1 && remainder != 0 {
		pieceLength = remainder
	}

	if blockBegin < 0 || blockBegin >= pieceLength {
		return &InvalidMessageError{Message: "Block start offset (" + strconv.Itoa(blockBegin) + ") out of bounds [0, " + strconv.Itoa(pieceLength) + ")", MsgType: "REQUEST_MESSAGE"}
	}

	if blockLen == 0 {
		return &InvalidMessageError{Message: "Request has block length 0", MsgType: "REQUEST_MESSAGE"}
	}

	if blockBegin+blockLen > pieceLength {
		return &InvalidMessageError{Message: "Requested piece portion []" + strconv.Itoa(blockBegin) + ", " + strconv.Itoa(blockBegin+blockLen) + ") is out of bounds [0, " + strconv.Itoa(pieceLength) + ")", MsgType: "REQUEST_MESSAGE"}
	}

	return nil
}
