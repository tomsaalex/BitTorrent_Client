package bencoding

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"

	bytesources "github.com/tomsaalex/BitTorrent_Client/bencoding/ByteSources"
	"github.com/tomsaalex/BitTorrent_Client/bencoding/bparserrs"
)

type Codec struct {
}

func (c *Codec) BencodeGeneral(sourceObject BencodableValue) (string, error) {
	return bencodeObject(sourceObject)
}

func bencodeObject(sourceObject BencodableValue) (string, error) {
	switch castValue := sourceObject.(type) {
	case BencodableInt:
		return bencodeInt(castValue), nil
	case BencodableString:
		return bencodeString(castValue), nil
	case BencodableList:
		return bencodeList(castValue)
	case BencodableMap:
		return bencodedDictionary(castValue)
	default:
		return "", &bparserrs.EncodingError{Message: "Tried to Bencode an incompatible object."}
	}
}

func bencodeInt(sourceInt BencodableInt) string {
	return "i" + strconv.Itoa(sourceInt) + "e"
}

func bencodeString(sourceString BencodableString) string {
	return strconv.Itoa(len(sourceString)) + ":" + sourceString
}

func bencodeList(sourceList BencodableList) (string, error) {
	var answerStringBuilder strings.Builder

	answerStringBuilder.WriteByte('l')

	for _, element := range sourceList {
		bencodedElement, bencodingError := bencodeObject(element)
		if bencodingError != nil {
			return "", &bparserrs.EncodingError{Message: "List element couldn't be Bencoded."}
		}

		answerStringBuilder.WriteString(bencodedElement)
	}

	answerStringBuilder.WriteByte('e')
	return answerStringBuilder.String(), nil
}

func bencodedDictionary(sourceDictionary BencodableMap) (string, error) {
	var answerStringBuilder strings.Builder

	answerStringBuilder.WriteByte('d')

	keys := make([]string, 0, len(sourceDictionary))
	for currentKey := range sourceDictionary {
		keys = append(keys, currentKey)
	}

	sort.Strings(keys)

	for _, currentKey := range keys {
		answerStringBuilder.WriteString(bencodeString(currentKey))

		bencodedValue, bencodingError := bencodeObject(sourceDictionary[currentKey])
		if bencodingError != nil {
			return "", &bparserrs.EncodingError{Message: "Dictionary value of key" + currentKey + "couldn't be Bencoded."}
		}

		answerStringBuilder.WriteString(bencodedValue)
	}

	answerStringBuilder.WriteByte('e')
	return answerStringBuilder.String(), nil
}

func (c *Codec) DecodeString(rawText string) (BencodableValue, error) {
	byteSource := &bytesources.StringSource{StringData: rawText, Position: 0}

	decodedString, decodingError := c.decode(byteSource)
	return decodedString, decodingError
}

func (c *Codec) DecodeFile(filePath string) (BencodableValue, error) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	bufferedReader := bufio.NewReader(file)

	byteSource := &bytesources.FileSource{BufferedReader: bufferedReader}

	decodedFile, decodingError := c.decode(byteSource)
	return decodedFile, decodingError
}

func (*Codec) decode(byteSource bytesources.ByteSource) (BencodableValue, error) {
	decodedSource, error := parseNext(byteSource)
	if error != nil {
		return nil, error
	}

	return decodedSource, nil
}

func parseNext(byteSource bytesources.ByteSource) (BencodableValue, error) {
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
		return "", &bparserrs.DecodingError{Message: "Source format doesn't respect Bencode restrictions"}
	}
}

func parseDictionary(byteSource bytesources.ByteSource) (BencodableMap, error) {
	decodedMap := make(map[string]any)
	var nextKey BencodableValue
	var bencodedStringKey BencodableString
	var nextValue BencodableValue
	var err error
	var keyIsString bool
	var currentByte byte
	byteSource.Read()

	for {
		currentByte, err = byteSource.Peek()
		if err != nil {
			return BencodableMap{}, err
		}

		if currentByte == 'e' {
			byteSource.Read()
			return decodedMap, nil
		} else {
			nextKey, err = parseNext(byteSource)
			if err != nil {
				return BencodableMap{}, err
			}
			bencodedStringKey, keyIsString = nextKey.(BencodableString)

			if !keyIsString {
				return BencodableMap{}, &bparserrs.DecodingError{Message: "Expected dictionary key to be a string"}
			}

			nextValue, err = parseNext(byteSource)
			if err != nil {
				return BencodableMap{}, &bparserrs.DecodingError{Message: "Dictionary expected value for last key"}
			}

			decodedMap[bencodedStringKey] = nextValue
		}
	}
}

func parseList(byteSource bytesources.ByteSource) (BencodableList, error) {
	var decodedList []any
	var nextElement BencodableValue
	var err error
	var currentByte byte
	byteSource.Read()
	for {
		currentByte, err = byteSource.Peek()
		if err != nil {
			return BencodableList{}, err
		}

		if currentByte == 'e' {
			byteSource.Read()
			return decodedList, nil
		} else {
			nextElement, err = parseNext(byteSource)
			if err != nil {
				return BencodableList{}, err
			}

			decodedList = append(decodedList, nextElement)
		}
	}
}

func parseInt(byteSource bytesources.ByteSource) (BencodableInt, error) {
	decodedInt := 0
	isNegative := false
	emptyNumber := true
	byteSource.Read()
	for {
		currentByte, err := byteSource.Read()

		if err != nil {
			return 0, err
		}

		if currentByte == 'e' {
			if emptyNumber {
				return 0, &bparserrs.DecodingError{Message: "Nil integer found in source text"}
			}
			if isNegative {
				decodedInt = -decodedInt
			}
			return decodedInt, nil
		} else if currentByte == '-' {
			if emptyNumber && !isNegative {
				isNegative = true
			} else {
				return 0, &bparserrs.DecodingError{Message: "Malformed integer (extra minus sounds) found in source text"}
			}
		} else if currentByte >= '0' && currentByte <= '9' {
			decodedInt = decodedInt*10 + int(currentByte-'0')
			emptyNumber = false
		} else {
			return 0, &bparserrs.DecodingError{Message: "Malformed integer found in source text"}
		}
	}

}

func parseString(byteSource bytesources.ByteSource) (BencodableString, error) {
	stringLength := 0
	for {
		currentByte, err := byteSource.Read()
		if err != nil {
			return "", err
		}

		if currentByte >= '0' && currentByte <= '9' {
			stringLength = stringLength*10 + int(currentByte-'0')
		} else if currentByte == ':' {
			break
		} else {
			return "", &bparserrs.DecodingError{Message: "Malformed string length found in source text"}
		}
	}

	readString, err := byteSource.ReadMultiple(stringLength)

	if err != nil {
		return "", err
	}

	return string(readString), nil
}
