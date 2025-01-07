package torrentclient

import (
	"bytes"
	"crypto/sha1"
	"path/filepath"

	"github.com/tomsaalex/BitTorrent_Client/bencoding"
	"github.com/tomsaalex/BitTorrent_Client/customdatatypes"
)

type TorrentParser struct {
	c bencoding.Codec
}

func (tp *TorrentParser) ParseTorrentFile(filePath string) (TorrentData, error) {
	bencodedValue, err := tp.c.DecodeFile(filePath)

	if err != nil {
		return TorrentData{}, err
	}

	return tp.extractTorrentData(bencodedValue)
}

func calculateInfohash(bencodedInfoDict string) customdatatypes.CustomHash {
	h := sha1.New()
	h.Write([]byte(bencodedInfoDict))
	return customdatatypes.CustomHash{HashBytes: h.Sum(nil)}
}

func (tp *TorrentParser) extractTorrentData(rawTorrentData bencoding.BencodableValue) (TorrentData, error) {
	var newTorrentData TorrentData

	mainDictionaryValue, conversionSuccessful := rawTorrentData.(bencoding.BencodableMap)

	if !conversionSuccessful {
		return TorrentData{}, &MalformedTorrentError{Message: "File doesn't respect BitTorrent Specifications. Overall structure should be a dictionary."}
	}

	announce, valuePresent := mainDictionaryValue["announce"].(bencoding.BencodableString)
	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Announce missing"}
	}
	newTorrentData.Announce = announce

	createdBy, valuePresent := mainDictionaryValue["created by"].(bencoding.BencodableString)
	if valuePresent {
		newTorrentData.CreatedBy = createdBy
	}

	creationDate, valuePresent := mainDictionaryValue["creation date"].(bencoding.BencodableInt)
	if valuePresent {
		newTorrentData.CreationDate = creationDate
	}

	comment, valuePresent := mainDictionaryValue["comment"].(bencoding.BencodableString)
	if valuePresent {
		newTorrentData.Comment = comment
	}

	encoding, valuePresent := mainDictionaryValue["encoding"].(bencoding.BencodableString)
	if valuePresent {
		newTorrentData.Encoding = encoding
	}

	// Info Dictionary

	bencodedInfoDictionary, valuePresent := mainDictionaryValue["info"].(bencoding.BencodableMap)
	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Info dictionary missing"}
	}

	infoDictionary := bencodedInfoDictionary

	// Calculating infohash

	encodedInfoDictionary, encodingError := tp.c.BencodeGeneral(infoDictionary)

	if encodingError != nil {
		return TorrentData{}, &MalformedTorrentError{Message: "Couldn't reencode info dictionary to calculate torrent info hash"}
	}

	newTorrentData.Infohash = calculateInfohash(encodedInfoDictionary)

	// Common fields

	pieceLength, valuePresent := infoDictionary["piece length"].(bencoding.BencodableInt)
	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Piece length missing"}
	}

	newTorrentData.PieceLength = pieceLength

	pieces, valuePresent := infoDictionary["pieces"].(bencoding.BencodableString)

	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Pieces string missing"}
	}

	// Separating the concatenated hashes into an array of hashes
	piecesString := pieces
	var buf bytes.Buffer

	for i := 0; i < len(piecesString); i++ {
		buf.WriteByte(piecesString[i])
		if buf.Len() == 20 {
			bufCopy := make([]byte, buf.Len())
			copy(bufCopy, buf.Bytes())
			newTorrentData.PieceHashes = append(newTorrentData.PieceHashes, customdatatypes.CustomHash{HashBytes: bufCopy})
			buf.Reset()
		}
	}

	private, valuePresent := infoDictionary["private"].(bencoding.BencodableInt)

	if valuePresent {
		newTorrentData.Private = int8(private)
	}

	// Single File Mode fields

	name, valuePresent := infoDictionary["name"].(bencoding.BencodableString)

	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Name missing"}
	}

	newTorrentData.Name = name

	fileLength, valuePresent := infoDictionary["length"].(bencoding.BencodableInt)

	if valuePresent {
		newTorrentData.FileLength = fileLength
		return newTorrentData, nil // We're in Single File Mode, nothing else that follows matters
	}

	// Multiple Files Mode fields

	filesList, valuePresent := infoDictionary["files"].(bencoding.BencodableList)

	if !valuePresent {
		return TorrentData{}, &MalformedTorrentError{Message: "Files missing in Multiple File Mode"}
	}

	torrentSize := 0
	for _, currentDictionary := range filesList {
		regularDictionary, valuePresent := currentDictionary.(bencoding.BencodableMap)
		if !valuePresent {
			return TorrentData{}, &MalformedTorrentError{Message: "Files doesn't contain properly Bencoded dictionaries."}
		}

		var currentFileData FileData

		length, valuePresent := regularDictionary["length"].(bencoding.BencodableInt)

		if !valuePresent {
			return TorrentData{}, &MalformedTorrentError{Message: "A dictionary in Files doesn't contain a file's length"}
		}

		currentFileData.FileLength = length
		torrentSize += length

		path, valuePresent := regularDictionary["path"].(bencoding.BencodableList)

		if !valuePresent {
			return TorrentData{}, &MalformedTorrentError{Message: "A dictionary in Files doesn't contain the file's path"}
		}

		pathFrags := make([]string, 0)
		for _, pathFragment := range path {
			stringPathFragment, valuePresent := pathFragment.(bencoding.BencodableString)

			if !valuePresent {
				return TorrentData{}, &MalformedTorrentError{Message: "Path fragments in a File dictionary aren't strings"}
			}

			pathFrags = append(pathFrags, stringPathFragment)

		}
		currentFileData.FilePath = filepath.Join(pathFrags...)
		newTorrentData.Files = append(newTorrentData.Files, currentFileData)
	}

	newTorrentData.TorrentSize = torrentSize
	return newTorrentData, nil
}
