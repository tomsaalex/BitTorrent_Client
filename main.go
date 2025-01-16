package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	/*
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)

		testTorrentClient := torrentclient.NewTorrentClient()

		testTorrentClient.AddTorrent("torrent_files/Atherton - Rivers of Fire.mp3.torrent")

		// Absolutely not how this should work, but it'll do until the proper implementation of the program closing logic is written.
		var wg sync.WaitGroup
		wg.Add(1)
		wg.Wait()
	*/

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "BitTorrent_Client",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		//BackgroundColour: &options.RGBA{R: 79, G: 52, B: 90, A: 1},
		//Windows: &windows.Options{
		//	WebviewIsTransparent: false,
		//	WindowIsTranslucent:  true,
		//},

		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}

}

/*
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
*/
