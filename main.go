package main

import (
	"log/slog"
	"os"
	"strconv"
	"sync"

	"github.com/tomsaalex/BitTorrent_Client/bencoding"
	"github.com/tomsaalex/BitTorrent_Client/bencoding/bparserrs"
	"github.com/tomsaalex/BitTorrent_Client/torrentclient"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	testTorrentClient := torrentclient.NewTorrentClient()
	//testTorrentClient.AddTorrent("Atherton - Rivers of Fire.mp3.torrent")
	testTorrentClient.AddTorrent("dummy.torrent")

	// Absolutely not how this should work, but it'll do until the proper implementation of the program closing logic is written.
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}

func stringifyBencodedValue(bencodedValue bencoding.BencodableValue) (string, error) {
	stringOutput := ""
	switch castValue := bencodedValue.(type) {
	case bencoding.BencodableInt:
		stringOutput = strconv.Itoa(castValue)
	case bencoding.BencodableString:
		stringOutput = castValue
	case bencoding.BencodableList:
		stringOutput += "{"
		for _, value := range castValue {
			stringValue, err := stringifyBencodedValue(value)
			if err != nil {
				return "", err
			}
			stringOutput += stringValue + ","
		}
		stringOutput = stringOutput[:len(stringOutput)-2]
		stringOutput += "}"
	case bencoding.BencodableMap:
		stringOutput += "["
		for key, value := range castValue {
			stringValue, err := stringifyBencodedValue(value)
			if err != nil {
				return "", err
			}
			stringOutput += key + ":" + stringValue + ""
		}
		stringOutput += "]"
	default:
		return "", &bparserrs.DecodingError{Message: "Argument isn't a known BencodedValue type"}
	}
	return stringOutput, nil
}
