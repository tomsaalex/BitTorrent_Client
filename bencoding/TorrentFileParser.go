package bencoding

import (
	"crypto/sha1"
	"strings"

	torrentparsingerrors "github.com/tomsaalex/BitTorrent_Client/bencoding/torrent_parsing_errors"
	"github.com/tomsaalex/BitTorrent_Client/customdatatypes"
)

type TorrentParser struct {
	c Codec
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

func (tp *TorrentParser) extractTorrentData(rawTorrentData BencodableValue) (TorrentData, error) {
	var newTorrentData TorrentData

	mainDictionaryValue, conversionSuccessful := rawTorrentData.(BencodableMap)

	if !conversionSuccessful {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "File doesn't respect BitTorrent Specifications. Overall structure should be a dictionary."}
	}

	announce, valuePresent := mainDictionaryValue["announce"].(BencodableString)
	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Announce missing"}
	}
	newTorrentData.Announce = announce

	createdBy, valuePresent := mainDictionaryValue["created by"].(BencodableString)
	if valuePresent {
		newTorrentData.CreatedBy = createdBy
	}

	creationDate, valuePresent := mainDictionaryValue["creation date"].(BencodableInt)
	if valuePresent {
		newTorrentData.CreationDate = creationDate
	}

	comment, valuePresent := mainDictionaryValue["comment"].(BencodableString)
	if valuePresent {
		newTorrentData.Comment = comment
	}

	encoding, valuePresent := mainDictionaryValue["encoding"].(BencodableString)
	if valuePresent {
		newTorrentData.Encoding = encoding
	}

	// Info Dictionary

	bencodedInfoDictionary, valuePresent := mainDictionaryValue["info"].(BencodableMap)
	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Info dictionary missing"}
	}

	infoDictionary := bencodedInfoDictionary

	// Calculating infohash

	encodedInfoDictionary, encodingError := tp.c.BencodeGeneral(infoDictionary)

	if encodingError != nil {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Couldn't reencode info dictionary to calculate torrent info hash"}
	}

	newTorrentData.Infohash = calculateInfohash(encodedInfoDictionary)

	// Common fields

	pieceLength, valuePresent := infoDictionary["piece length"].(BencodableInt)
	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Piece length missing"}
	}

	newTorrentData.PieceLength = pieceLength

	pieces, valuePresent := infoDictionary["pieces"].(BencodableString)

	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Pieces string missing"}
	}

	// Separating the concatenated hashes into an array of hashes
	piecesString := pieces
	var sb strings.Builder

	for i := 0; i < len(piecesString); i++ {
		sb.WriteByte(piecesString[i])
		if sb.Len() == 20 {
			newTorrentData.PieceHashes = append(newTorrentData.PieceHashes, sb.String())
			sb.Reset()
		}
	}

	private, valuePresent := infoDictionary["private"].(BencodableInt)

	if valuePresent {
		newTorrentData.Private = int8(private)
	}

	// Single File Mode fields

	name, valuePresent := infoDictionary["name"].(BencodableString)

	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Name missing"}
	}

	newTorrentData.Name = name

	fileLength, valuePresent := infoDictionary["length"].(BencodableInt)

	if valuePresent {
		newTorrentData.FileLength = fileLength
		return newTorrentData, nil // We're in Single File Mode, nothing else that follows matters
	}

	filesList, valuePresent := infoDictionary["files"].(BencodableList)

	if !valuePresent {
		return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Files missing in Multiple File Mode"}
	}

	for _, currentDictionary := range filesList {
		regularDictionary, valuePresent := currentDictionary.(BencodableMap)
		if !valuePresent {
			return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Files doesn't contain properly Bencoded dictionaries."}
		}

		var currentFileData FileData

		length, valuePresent := regularDictionary["length"].(BencodableInt)

		if !valuePresent {
			return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "A dictionary in Files doesn't contain a file's length"}
		}

		currentFileData.FileLength = length

		path, valuePresent := regularDictionary["path"].(BencodableList)

		if !valuePresent {
			return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "A dictionary in Files doesn't contain the file's path"}
		}

		sb.Reset()
		for index, pathFragment := range path {
			stringPathFragment, valuePresent := pathFragment.(BencodableString)

			if !valuePresent {
				return TorrentData{}, &torrentparsingerrors.MalformedTorrentError{Message: "Path fragments in a File dictionary aren't strings"}
			}

			sb.WriteString(stringPathFragment)
			if index != len(path)-1 {
				sb.WriteByte('/')
			}
		}

		currentFileData.FilePath = sb.String()

		newTorrentData.Files = append(newTorrentData.Files, currentFileData)
	}

	return newTorrentData, nil
}
