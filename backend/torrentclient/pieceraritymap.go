package torrentclient

import (
	"math/rand/v2"
	"sort"

	generalerrors "github.com/tomsaalex/BitTorrent_Client/backend/GeneralErrors"
)

type PieceRarityMap struct {
	frequencyMap     map[int]int
	cachedPieceSlice []int
	cacheValid       bool
}

func NewPieceRarityMap() *PieceRarityMap {
	frequencyMap := make(map[int]int)
	cacheValid := false
	return &PieceRarityMap{frequencyMap: frequencyMap, cacheValid: cacheValid}
}

func (pm *PieceRarityMap) incrementPieceAvailability(pieceIndex int) {
	oldValue, found := pm.frequencyMap[pieceIndex]

	pm.cacheValid = false
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

	pm.cacheValid = false
	if oldValue == 1 {
		delete(pm.frequencyMap, pieceIndex)
		return nil
	}

	pm.frequencyMap[pieceIndex] = oldValue - 1
	return nil
}

func (pm *PieceRarityMap) removePiece(pieceIndex int) {
	delete(pm.frequencyMap, pieceIndex)

	removedPieceLocation := -1
	for i, piece := range pm.cachedPieceSlice {
		if piece == pieceIndex {
			removedPieceLocation = i
			break
		}
	}
	if removedPieceLocation != -1 {
		pm.cachedPieceSlice = append(pm.cachedPieceSlice[:removedPieceLocation], pm.cachedPieceSlice[removedPieceLocation+1:]...)
	}
}

func (pm *PieceRarityMap) rebuildCache() {
	keys := make([]int, len(pm.frequencyMap))

	i := 0
	for k := range pm.frequencyMap {
		keys[i] = k
		i++
	}

	sort.Slice(keys, func(i int, j int) bool {
		return pm.frequencyMap[keys[i]] < pm.frequencyMap[keys[j]]
	})

	pm.cachedPieceSlice = keys
	pm.cacheValid = true
}

func (pm *PieceRarityMap) getRarePieceIndex() int {
	if len(pm.frequencyMap) == 0 {
		return -1
	}

	if !pm.cacheValid {
		pm.rebuildCache()
	}

	randRange := 20
	if len(pm.cachedPieceSlice) < randRange {
		randRange = len(pm.cachedPieceSlice)
	}

	chosenIndex := rand.IntN(randRange)
	chosenPiece := pm.cachedPieceSlice[chosenIndex]
	pm.cachedPieceSlice = append(pm.cachedPieceSlice[:chosenIndex], pm.cachedPieceSlice[chosenIndex+1:]...)
	delete(pm.frequencyMap, chosenPiece)

	return chosenPiece
}
