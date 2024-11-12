package Bencoding

import (
	"strings"

	"github.com/tomsaalex/BitTorrent_Client/Bencoder/TorrentParsingErrors"
)

type TorrentParser struct {
	c codec
}

func (tp *TorrentParser) ParseTorrentFile(filePath string) (TorrentData, error) {
	bencodedValue, err := tp.c.DecodeFile(filePath)

	if err != nil {
		return TorrentData{}, err
	}

	return tp.extractTorrentData(bencodedValue)
}

func (*TorrentParser) extractTorrentData(rawTorrentData BencodedValue) (TorrentData, error) {
	var newTorrentData TorrentData

	mainDictionaryValue, conversionSuccessful := rawTorrentData.(BencodedMap)

	if !conversionSuccessful {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "File doesn't respect BitTorrent Specifications. Overall structure should be a dictionary."}
	}

	announce, valuePresent := mainDictionaryValue.MapValue["announce"].(BencodedString)
	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Announce missing"}
	}
	newTorrentData.Announce = announce.StringValue

	createdBy, valuePresent := mainDictionaryValue.MapValue["created by"].(BencodedString)
	if valuePresent {
		newTorrentData.CreatedBy = createdBy.StringValue
	}

	creationDate, valuePresent := mainDictionaryValue.MapValue["creation date"].(BencodedInt)
	if valuePresent {
		newTorrentData.CreationDate = creationDate.IntValue
	}

	comment, valuePresent := mainDictionaryValue.MapValue["comment"].(BencodedString)
	if valuePresent {
		newTorrentData.Comment = comment.StringValue
	}

	encoding, valuePresent := mainDictionaryValue.MapValue["encoding"].(BencodedString)
	if valuePresent {
		newTorrentData.Encoding = encoding.StringValue
	}

	// Info Dictionary

	bencodedInfoDictionary, valuePresent := mainDictionaryValue.MapValue["info"].(BencodedMap)
	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Info dictionary missing"}
	}

	infoDictionary := bencodedInfoDictionary.MapValue

	// Common fields

	pieceLength, valuePresent := infoDictionary["piece length"].(BencodedInt)
	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Piece length missing"}
	}

	newTorrentData.PieceLength = pieceLength.IntValue

	pieces, valuePresent := infoDictionary["pieces"].(BencodedString)

	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Pieces string missing"}
	}

	// Separating the concatenated hashes into an array of hashes
	piecesString := pieces.StringValue
	var sb strings.Builder

	for i := 0; i < len(piecesString); i++ {
		sb.WriteByte(piecesString[i])
		if sb.Len() == 20 {
			newTorrentData.PieceHashes = append(newTorrentData.PieceHashes, sb.String())
			sb.Reset()
		}
	}

	private, valuePresent := infoDictionary["private"].(BencodedInt)

	if valuePresent {
		newTorrentData.Private = int8(private.IntValue)
	}

	// Single File Mode fields

	name, valuePresent := infoDictionary["name"].(BencodedString)

	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Name missing"}
	}

	newTorrentData.Name = name.StringValue

	fileLength, valuePresent := infoDictionary["length"].(BencodedInt)

	if valuePresent {
		newTorrentData.FileLength = fileLength.IntValue
		return newTorrentData, nil // We're in Single File Mode, nothing else that follows matters
	}

	filesList, valuePresent := infoDictionary["files"].(BencodedList)

	if !valuePresent {
		return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Files missing in Multiple File Mode"}
	}

	for _, currentDictionary := range filesList.ListValue {
		regularDictionary, valuePresent := currentDictionary.(BencodedMap)
		if !valuePresent {
			return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Files doesn't contain properly Bencoded dictionaries."}
		}

		var currentFileData FileData

		length, valuePresent := regularDictionary.MapValue["length"].(BencodedInt)

		if !valuePresent {
			return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "A dictionary in Files doesn't contain a file's length"}
		}

		currentFileData.FileLength = length.IntValue

		path, valuePresent := regularDictionary.MapValue["path"].(BencodedList)

		if !valuePresent {
			return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "A dictionary in Files doesn't contain the file's path"}
		}

		sb.Reset()
		for index, pathFragment := range path.ListValue {
			stringPathFragment, valuePresent := pathFragment.(BencodedString)

			if !valuePresent {
				return TorrentData{}, &TorrentParsingErrors.MalformedTorrentError{Message: "Path fragments in a File dictionary aren't strings"}
			}

			sb.WriteString(stringPathFragment.StringValue)
			if index != len(path.ListValue)-1 {
				sb.WriteByte('/')
			}
		}

		currentFileData.FilePath = sb.String()

		newTorrentData.Files = append(newTorrentData.Files, currentFileData)
	}

	return newTorrentData, nil
}
