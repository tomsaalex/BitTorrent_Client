package torrentclient

import generalerrors "github.com/tomsaalex/BitTorrent_Client/backend/GeneralErrors"

type PieceRarityMap struct {
	frequencyMap map[int]int
}

func NewPieceRarityMap() *PieceRarityMap {
	frequencyMap := make(map[int]int)
	return &PieceRarityMap{frequencyMap: frequencyMap}
}

func (pm *PieceRarityMap) incrementPieceAvailability(pieceIndex int) {
	oldValue, found := pm.frequencyMap[pieceIndex]
	if !found {
		pm.frequencyMap[pieceIndex] = 1
		return
	}

	pm.frequencyMap[pieceIndex] = oldValue + 1
}

func (pm *PieceRarityMap) decrementPieceAvailability(pieceIndex int) error {
	oldValue, found := pm.frequencyMap[pieceIndex]
	if !found {
		return &generalerrors.IllegalAccessError{Message: "Piece availability cannot be decreased. It is already 0."}
	}

	if oldValue == 1 {
		delete(pm.frequencyMap, pieceIndex)
		return nil
	}

	pm.frequencyMap[pieceIndex] = oldValue - 1

	return nil
}

func (pm *PieceRarityMap) removePiece(pieceIndex int) {
	delete(pm.frequencyMap, pieceIndex)
}

func (pm *PieceRarityMap) getRarestPieceIndex() int {
	// TODO: This could be done much faster if we used an ordered map instead of a regular map. Check if trade offs with insert/remove speed is worth it.
	// TODO: Always selecting the rarest piece isn't a good strategy, as it creates needless contention on that one piece if every peer does the same. It would be wise to add some randomness.

	desiredIndex := -1
	currentAvailability := -1

	for pieceIndex, availability := range pm.frequencyMap {
		if availability < currentAvailability || desiredIndex == -1 {
			desiredIndex = pieceIndex
			currentAvailability = availability
		}
	}

	return desiredIndex
}
