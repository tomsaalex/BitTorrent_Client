package torrentclient

import "os"

func createSparseFile(tData *TorrentData) (*os.File, error) {
	file, err := os.Create(tData.Name)
	if err != nil {
		return nil, err
	}

	// Set the file size without writing zeroes
	err = file.Truncate(int64(tData.FileLength))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func writePieceToFile(file *os.File, newPiece piece) error {
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
}

// TODO: This entire file is written with the assumption of a torrent with only one file. For now.
func fileWriter(pieceInput <-chan piece, tData *TorrentData) {
	file, err := createSparseFile(tData)
	defer file.Close()
	if err != nil {
		// TODO: Handle this more gracefully
		panic(err)
	}

	for {
		select {
		case newPiece := <-pieceInput:
			writePieceToFile(file, newPiece)
		}
	}
}
