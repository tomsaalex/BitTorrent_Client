package Bencoding

import (
	"bufio"
	"os"

	"github.com/tomsaalex/BitTorrent_Client/Bencoder/ByteSources"
	"github.com/tomsaalex/BitTorrent_Client/Bencoder/ParsingErrors"
)

type codec struct {
}

func (*codec) Encode(rawText string) (string, error) {
	return "text", nil
}

func (c *codec) DecodeString(rawText string) (BencodedValue, error) {
	byteSource := &ByteSources.StringSource{StringData: rawText, Position: 0}

	decodedString, decodingError := c.decode(byteSource)
	return decodedString, decodingError
}

func (c *codec) DecodeFile(filePath string) (BencodedValue, error) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	bufferedReader := bufio.NewReader(file)

	byteSource := &ByteSources.FileSource{BufferedReader: bufferedReader}

	decodedFile, decodingError := c.decode(byteSource)
	return decodedFile, decodingError
}

func (*codec) decode(byteSource ByteSources.ByteSource) (BencodedValue, error) {
	decodedSource, error := parseNext(byteSource)
	if error != nil {
		return nil, error
	}

	return decodedSource, nil
}

func parseNext(byteSource ByteSources.ByteSource) (BencodedValue, error) {
	currentByte, error := byteSource.Peek()
	if error != nil {
		return "", error
	}

	switch {
	case currentByte == 'd':
		return parseDictionary(byteSource)
	case currentByte == 'l':
		return parseList(byteSource)
	case currentByte == 'i':
		return parseInt(byteSource)
	case currentByte >= '0' && currentByte <= '9':
		return parseString(byteSource)
	default:
		return "", &ParsingErrors.DecodingError{Message: "Source format doesn't respect Bencode restrictions"}
	}
}

func parseDictionary(byteSource ByteSources.ByteSource) (BencodedMap, error) {
	decodedMap := make(map[string]BencodedValue)
	var nextKey BencodedValue
	var bencodedStringKey BencodedString
	var nextValue BencodedValue
	var err error
	var keyIsString bool
	var currentByte byte
	byteSource.Read()

	for {
		currentByte, err = byteSource.Peek()
		if err != nil {
			return BencodedMap{}, err
		}

		if currentByte == 'e' {
			byteSource.Read()
			return BencodedMap{decodedMap}, nil
		} else {
			nextKey, err = parseNext(byteSource)
			if err != nil {
				return BencodedMap{nil}, err
			}
			bencodedStringKey, keyIsString = nextKey.(BencodedString)

			if !keyIsString {
				return BencodedMap{}, &ParsingErrors.DecodingError{Message: "Expected dictionary key to be a string"}
			}

			nextValue, err = parseNext(byteSource)
			if err != nil {
				return BencodedMap{nil}, &ParsingErrors.DecodingError{Message: "Dictionary expected value for last key"}
			}

			decodedMap[bencodedStringKey.StringValue] = nextValue
		}
	}
}

func parseList(byteSource ByteSources.ByteSource) (BencodedList, error) {
	var decodedList []BencodedValue
	var nextElement BencodedValue
	var err error
	var currentByte byte
	byteSource.Read()
	for {
		currentByte, err = byteSource.Peek()
		if err != nil {
			return BencodedList{nil}, err
		}

		if currentByte == 'e' {
			byteSource.Read()
			return BencodedList{decodedList}, nil
		} else {
			nextElement, err = parseNext(byteSource)
			if err != nil {
				return BencodedList{nil}, err
			}

			decodedList = append(decodedList, nextElement)
		}
	}
}

func parseInt(byteSource ByteSources.ByteSource) (BencodedInt, error) {
	decodedInt := 0
	isNegative := false
	emptyNumber := true
	byteSource.Read()
	for {
		currentByte, err := byteSource.Read()

		if err != nil {
			return BencodedInt{0}, err
		}

		if currentByte == 'e' {
			if emptyNumber {
				return BencodedInt{0}, &ParsingErrors.DecodingError{Message: "Nil integer found in source text"}
			}
			if isNegative {
				decodedInt = -decodedInt
			}
			return BencodedInt{decodedInt}, nil
		} else if currentByte == '-' {
			if emptyNumber && !isNegative {
				isNegative = true
			} else {
				return BencodedInt{0}, &ParsingErrors.DecodingError{Message: "Malformed integer (extra minus sounds) found in source text"}
			}
		} else if currentByte >= '0' && currentByte <= '9' {
			decodedInt = decodedInt*10 + int(currentByte-'0')
			emptyNumber = false
		} else {
			return BencodedInt{0}, &ParsingErrors.DecodingError{Message: "Malformed integer found in source text"}
		}
	}

}

func parseString(byteSource ByteSources.ByteSource) (BencodedString, error) {
	stringLength := 0
	for {
		currentByte, err := byteSource.Read()
		if err != nil {
			return BencodedString{""}, err
		}

		if currentByte >= '0' && currentByte <= '9' {
			stringLength = stringLength*10 + int(currentByte-'0')
		} else if currentByte == ':' {
			break
		} else {
			return BencodedString{""}, &ParsingErrors.DecodingError{Message: "Malformed string length found in source text"}
		}
	}

	readString, err := byteSource.ReadMultiple(stringLength)

	if err != nil {
		return BencodedString{""}, err
	}

	return BencodedString{string(readString)}, nil
}
