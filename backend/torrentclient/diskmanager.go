package torrentclient

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

const TEMP_DOWNLOAD_LOCATION = "download_location/"

type pieceValidationResult struct {
	pieceIndex int
	valid      bool
}

type BlockRetrievalRequest struct {
	req         requestMessage
	pieceOutput chan peerMessage
}

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
	// Open file for reading
	file, err := os.OpenFile(actualPath, os.O_RDWR, 0777)
	if err != nil {
		return &IOError{Message: "Couldn't open file: " + actualPath}
	}
	defer file.Close()

	// Seek to the correct position
	_, err = file.Seek(pm.fileOffset, io.SeekStart)
	if err != nil {
		return &IOError{Message: "Couldn't seek in file: " + actualPath}
	}

	// Write the piece data
	dataForThisFile := pm.p.data[pm.pieceOffsetStart:pm.pieceOffsetEnd]
	_, err = file.Write(dataForThisFile)
	if err != nil {
		return &IOError{Message: "Couldn't write to file: " + actualPath}
	}

	return nil
}

func (dm *DiskManager) readPieceMappingFromFile(actualPath string, pm PieceFileMapping) ([]byte, error) {
	// Open file for reading
	file, err := os.OpenFile(actualPath, os.O_RDONLY, 0777)
	if err != nil {
		return []byte{}, &IOError{Message: "Couldn't open file: " + actualPath}
	}
	defer file.Close()

	// Seek to the correct position
	_, err = file.Seek(pm.fileOffset, io.SeekStart)
	if err != nil {
		return []byte{}, &IOError{Message: "Couldn't seek in file: " + actualPath}
	}

	fileBuf := make([]byte, pm.pieceOffsetEnd-pm.pieceOffsetStart)
	_, readErr := io.ReadFull(file, fileBuf)
	if readErr != nil {
		return []byte{}, &IOError{Message: "Couldn't read from file: " + actualPath}
	}

	return fileBuf, nil
}

func (dm *DiskManager) MapPieceToFiles(p piece, tData *TorrentData) []PieceFileMapping {
	// TODO: This entire function needs some thorough testing. Pieces spanning across 3 files, for example, strange behaviour around the last piece of the torrent etc.

	var coveredPieces float64
	dataAmountCovered := int64(0)
	reachedPieceLocation := false

	pieceOffset := int64(0)
	pieceFileMappings := make([]PieceFileMapping, 0)

	// This is done instead of using len(p.data) because we want to be able to generate mappings for block requests too
	// But we still need the reference to the piece so we can put it in the mappings.
	pieceLength := tData.PieceLength
	remainder := tData.TorrentSize % tData.PieceLength
	if p.pieceIndex == len(tData.PieceHashes)-1 && remainder != 0 {
		pieceLength = remainder
	}

	if len(tData.Files) == 0 {
		// TODO: The offset formula was used for single files, but it feels like it should fail. For the last piece it uses a length lower
		// than of the previous pieces to tell where it needs to start. This feels like it should lead to overlapping... Check with a hash if the files really make it across.
		fileOffset := int64(p.pieceIndex * tData.PieceLength)
		pieceMapping := PieceFileMapping{p: p, filePath: tData.Name, fileOffset: fileOffset, pieceOffsetStart: 0, pieceOffsetEnd: int64(pieceLength)}
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
			if int64(pieceLength)-pieceOffset > int64(file.FileLength)-fileOffset {
				pieceOffsetEnd = pieceOffsetStart + (int64(file.FileLength) - fileOffset)
			} else {
				pieceOffsetEnd = int64(pieceLength)
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

func (dm *DiskManager) fileWriter(ctx context.Context, pieceInput <-chan piece, pieceStoredAnnounce chan<- int, requestsInput <-chan BlockRetrievalRequest, tData *TorrentData) {
	fileIndex := make(map[string]bool)
	var directoryName string
	if len(tData.Files) > 0 {
		directoryName = tData.Name
	} else {
		directoryName = ""
	}

	for {
		select {
		case piece := <-pieceInput:
			pieceMappings := dm.MapPieceToFiles(piece, tData)

			for _, pm := range pieceMappings {
				joinedPath := filepath.Join(TEMP_DOWNLOAD_LOCATION, directoryName, pm.filePath)
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
		case retrievalRequest := <-requestsInput:
			// TODO: This is WILDLY inefficient. It reads an entire piece just to request a block from it.
			// The solution is to generate file mappings per block, I suppose. Refactor later.
			emptyPiece := piece{pieceIndex: retrievalRequest.req.index}
			pieceFileMappings := dm.MapPieceToFiles(emptyPiece, tData)

			retrievedBlock := make([]byte, 0, retrievalRequest.req.length)

			for _, pm := range pieceFileMappings {
				joinedPath := filepath.Join(TEMP_DOWNLOAD_LOCATION, directoryName, pm.filePath)
				partialBlock, err := dm.readPieceMappingFromFile(joinedPath, pm)

				if err != nil {
					// TODO: Handle this error better
					panic(err)
				}

				retrievedBlock = append(retrievedBlock, partialBlock...)
			}

			requestedBlock := retrievedBlock[retrievalRequest.req.begin : retrievalRequest.req.begin+retrievalRequest.req.length]
			pieceMessage := pieceMessage{index: retrievalRequest.req.index, begin: retrievalRequest.req.begin, block: requestedBlock}
			retrievalRequest.pieceOutput <- pieceMessage
		case <-ctx.Done():
			fmt.Println("Exitted out of fileWriter")
			return
		}
	}
}

func (dm *DiskManager) pieceFileMappingsForTorrent(tData TorrentData) []PieceFileMapping {
	pieceFileMappings := make([]PieceFileMapping, 0)

	pieceLength := tData.PieceLength
	remainder := tData.TorrentSize % len(tData.PieceHashes)
	if len(tData.Files) == 0 {
		for i, _ := range tData.PieceHashes {
			p := piece{pieceIndex: i, data: nil}

			if p.pieceIndex == len(tData.PieceHashes)-1 && remainder != 0 {
				pieceLength = remainder
			}

			fileOffset := int64(p.pieceIndex * tData.PieceLength)
			pieceMapping := PieceFileMapping{p: p, filePath: tData.Name, fileOffset: fileOffset, pieceOffsetStart: 0, pieceOffsetEnd: int64(pieceLength)}
			pieceFileMappings = append(pieceFileMappings, pieceMapping)

		}
		return pieceFileMappings
	}

	var fileOffset int64
	var pieceOffset int64 = 0
	pieceIndex := 0

	for _, file := range tData.Files {
		fileOffset = 0

		for fileOffset < int64(file.FileLength) {
			if file.FileLength-int(fileOffset) >= pieceLength-int(pieceOffset) {
				p := piece{pieceIndex: pieceIndex, data: nil}

				newMapping := PieceFileMapping{p: p, filePath: file.FilePath, fileOffset: fileOffset, pieceOffsetStart: int64(pieceOffset), pieceOffsetEnd: int64(pieceOffset) + int64(pieceLength-int(pieceOffset))}
				pieceFileMappings = append(pieceFileMappings, newMapping)

				fileOffset += int64(pieceLength - int(pieceOffset))
				pieceOffset = 0
				pieceIndex++
			} else {
				p := piece{pieceIndex: pieceIndex, data: nil}

				newPieceOffset := pieceOffset + int64(file.FileLength-int(fileOffset))
				newMapping := PieceFileMapping{p: p, filePath: file.FilePath, fileOffset: fileOffset, pieceOffsetStart: int64(pieceOffset), pieceOffsetEnd: newPieceOffset}
				pieceFileMappings = append(pieceFileMappings, newMapping)

				fileOffset += int64(file.FileLength - int(fileOffset))
				pieceOffset = newPieceOffset
			}
		}

	}

	return pieceFileMappings
}

func (dm *DiskManager) checkTorrentIntegrity(tData TorrentData, res chan<- pieceValidationResult) {
	// This won't be implemented using the MapPieceToFiles as it would mean looping over all the files for each piece.
	// That's insanely inefficient.

	var directoryName string
	if len(tData.Files) > 0 {
		directoryName = tData.Name
	} else {
		directoryName = ""
	}

	pieceFileMappings := dm.pieceFileMappingsForTorrent(tData)
	pieceBuffer := make([]byte, 0)

	prevBadFilePath := ""
	for i, pieceMapping := range pieceFileMappings {
		joinedPath := filepath.Join(TEMP_DOWNLOAD_LOCATION, directoryName, pieceMapping.filePath)
		if joinedPath == prevBadFilePath {
			continue
		}
		data, _ := dm.readPieceMappingFromFile(joinedPath, pieceMapping)

		pieceBuffer = append(pieceBuffer, data...)

		if i < len(pieceFileMappings)-1 && pieceFileMappings[i+1].p.pieceIndex != pieceMapping.p.pieceIndex ||
			i == len(pieceFileMappings)-1 {
			pieceValid := dm.pieceValid(&tData, piece{pieceIndex: pieceMapping.p.pieceIndex, data: pieceBuffer})

			res <- pieceValidationResult{pieceIndex: pieceMapping.p.pieceIndex, valid: pieceValid}
			pieceBuffer = make([]byte, 0)
		}
	}

	close(res)
}

func (dm *DiskManager) pieceValid(tData *TorrentData, p piece) bool {
	var sha = sha1.New()
	sha.Write(p.data)
	pieceHash := sha.Sum(nil)[:20]

	hashCorrect := bytes.Equal(pieceHash, tData.PieceHashes[p.pieceIndex].HashBytes)
	return hashCorrect
}
