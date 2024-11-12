package main

import (
	"fmt"
	"strconv"

	Bencoding "github.com/tomsaalex/BitTorrent_Client/Bencoder"
	"github.com/tomsaalex/BitTorrent_Client/Bencoder/ParsingErrors"
)

func main() {
	t := Bencoding.TorrentParser{}
	//decodedValue, err := b.DecodeString("ld13:chill_examplell2:abi34ee3:abce2:xDi45ee3:loli-69ee")

	decodedValue, err := t.ParseTorrentFile("Harry Potter (Wizarding World) Series.torrent")

	if err != nil {
		fmt.Print(err)
	} else {
		stringValue, err := stringifyBencodedValue(decodedValue)
		if err != nil {
			fmt.Print(err)
		}
		fmt.Print(stringValue)
	}
}

func stringifyBencodedValue(bencodedValue Bencoding.BencodedValue) (string, error) {
	stringOutput := ""
	switch castValue := bencodedValue.(type) {
	case Bencoding.BencodedInt:
		stringOutput = strconv.Itoa(castValue.IntValue)
	case Bencoding.BencodedString:
		stringOutput = castValue.StringValue
	case Bencoding.BencodedList:
		stringOutput += "{"
		for _, value := range castValue.ListValue {
			stringValue, err := stringifyBencodedValue(value)
			if err != nil {
				return "", err
			}
			stringOutput += stringValue + ","
		}
		stringOutput = stringOutput[:len(stringOutput)-2]
		stringOutput += "}"
	case Bencoding.BencodedMap:
		stringOutput += "["
		for key, value := range castValue.MapValue {
			stringValue, err := stringifyBencodedValue(value)
			if err != nil {
				return "", err
			}
			stringOutput += key + ":" + stringValue + ""
		}
		stringOutput += "]"
	default:
		return "", &ParsingErrors.DecodingError{Message: "Argument isn't a known BencodedValue type"}
	}
	return stringOutput, nil
}
