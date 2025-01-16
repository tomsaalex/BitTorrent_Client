package torrentclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

type PieceFileMapping struct {
	p                piece
	filePath         string
	fileOffset       int64
	pieceOffsetStart int64
	pieceOffsetEnd   int64
}

type DiskManager struct {
}

func (dm *DiskManager) createSparseFile(path string, tData *TorrentData) (*os.File, error) {
	basePath := filepath.Dir(path)
	err := os.MkdirAll(basePath, 0777) // Unix style permissions. rwx for everybody

	if err != nil {
		return nil, &IOError{Message: "Couldn't create the following folder structure: " + basePath}
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, &IOError{Message: "Couldn't create file with path: " + path}
	}

	//defer file.Close()
	// Set the file size without writing zeroes
	err = file.Truncate(int64(tData.FileLength))
	if err != nil {
		return nil, &IOError{Message: "Couldn't set file size to " + strconv.Itoa(tData.FileLength) + " for file: " + path}
	}
	return file, nil
}

/*
func (dm *DiskManager) writePieceToFile(file *os.File, newPiece piece) error {
	// Calculate the byte offset for this piece
	offset := int64(newPiece.pieceIndex * len(newPiece.data))

	// Seek to the correct position
	_, err := file.Seek(offset, 0)
	if err != nil {
		return err
	}

	// Write the piece data
	_, err = file.Write(newPiece.data)

	return err
}*/

func (dm *DiskManager) writePieceMappingToFile(actualPath string, pm PieceFileMapping) error {
	joinedPath := filepath.Join(actualPath)

	// Open file for reading
	file, err := os.OpenFile(joinedPath, os.O_RDWR, 0777)
	if err != nil {
		return &IOError{Message: "Couldn't open file: " + joinedPath}
	}
	defer file.Close()

	// Seek to the correct position
	newOffset, err := file.Seek(pm.fileOffset, io.SeekStart)
	fmt.Print(newOffset)
	if err != nil {
		return &IOError{Message: "Couldn't seek in file: " + joinedPath}
	}

	// Write the piece data
	dataForThisFile := pm.p.data[pm.pieceOffsetStart:pm.pieceOffsetEnd]
	_, err = file.Write(dataForThisFile)
	if err != nil {
		return &IOError{Message: "Couldn't write to file: " + joinedPath}
	}

	return nil
}

func (dm *DiskManager) MapPieceToFiles(p piece, tData *TorrentData) []PieceFileMapping {
	// TODO: This entire function needs some thorough testing. Pieces spanning across 3 files, for example, strange behaviour around the last piece of the torrent etc.

	var coveredPieces float64
	dataAmountCovered := int64(0)
	reachedPieceLocation := false

	pieceOffset := int64(0)
	pieceFileMappings := make([]PieceFileMapping, 0)

	if len(tData.Files) == 0 {
		// TODO: The offset formula was used for single files, but it feels like it should fail. For the last piece it uses a length lower
		// than of the previous pieces to tell where it needs to start. This feels like it should lead to overlapping... Check with a hash if the files really make it across.
		fileOffset := int64(p.pieceIndex * tData.PieceLength)
		pieceMapping := PieceFileMapping{p: p, filePath: tData.Name, fileOffset: fileOffset, pieceOffsetStart: 0, pieceOffsetEnd: int64(len(p.data))}
		pieceFileMappings = append(pieceFileMappings, pieceMapping)
		return pieceFileMappings
	}

	for _, file := range tData.Files {
		dataAmountCovered += int64(file.FileLength)
		coveredPieces = float64(dataAmountCovered) / float64(tData.PieceLength)
		if coveredPieces > float64(p.pieceIndex) {
			reachedPieceLocation = true
		}

		if reachedPieceLocation {
			previousPiecesTotalSize := int64(p.pieceIndex) * int64(tData.PieceLength)
			fileOffset := previousPiecesTotalSize - (dataAmountCovered - int64(file.FileLength))
			if fileOffset < 0 {
				fileOffset = 0
			}
			pieceOffsetStart := pieceOffset
			var pieceOffsetEnd int64
			if int64(len(p.data))-pieceOffset > int64(file.FileLength)-fileOffset {
				pieceOffsetEnd = pieceOffsetStart + (int64(file.FileLength) - fileOffset)
			} else {
				pieceOffsetEnd = int64(len(p.data))
				reachedPieceLocation = false
			}

			newPieceFileMapping := PieceFileMapping{p: p, filePath: file.FilePath, fileOffset: fileOffset, pieceOffsetStart: pieceOffsetStart, pieceOffsetEnd: pieceOffsetEnd}
			pieceFileMappings = append(pieceFileMappings, newPieceFileMapping)

			pieceOffset = pieceOffsetEnd
			// TODO: I don't like it. Change the whole interval detection and exit mechanism
			if !reachedPieceLocation {
				break
			}
		}
	}

	return pieceFileMappings
}

func (dm *DiskManager) fileWriter(pieceInput <-chan piece, pieceStoredAnnounce chan<- int, tData *TorrentData) {
	fileIndex := make(map[string]bool)
	var directoryName string
	if len(tData.Files) > 0 {
		directoryName = tData.Name
	} else {
		directoryName = ""
	}

	for piece := range pieceInput {
		pieceMappings := dm.MapPieceToFiles(piece, tData)

		for _, pm := range pieceMappings {
			joinedPath := filepath.Join(directoryName, pm.filePath)
			_, fileExists := fileIndex[joinedPath]
			if !fileExists {
				var err error
				_, err = dm.createSparseFile(joinedPath, tData)
				fileIndex[joinedPath] = true
				if err != nil {
					panic(err)
				}
			}

			err := dm.writePieceMappingToFile(joinedPath, pm)
			if err != nil {
				// TODO: Handle this error better
				panic(err)
			}
		}

		slog.LogAttrs(
			context.Background(),
			slog.LevelInfo,
			"Wrote piece to disk",
			slog.Int("PieceIndex", piece.pieceIndex),
			slog.String("method", "diskManager"),
		)

		pieceStoredAnnounce <- piece.pieceIndex
	}
}
