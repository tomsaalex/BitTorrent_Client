package main

import (
	"context"
	"embed"
	"log/slog"
	"os"

	"github.com/tomsaalex/BitTorrent_Client/backend/torrentclient"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	/*logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	testTorrentClient := torrentclient.NewTorrentClient()

	go testTorrentClient.ClientRoutine()
	testTorrentClient.AddTorrent("torrent_files/Atherton - Rivers of Fire.mp3.torrent")

	// Absolutely not how this should work, but it'll do until the proper implementation of the program closing logic is written.
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()*/

	// Create an instance of the app structure
	app := NewApp()

	client := torrentclient.NewTorrentClient()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
			client.AppContext = ctx

			go client.ClientRoutine()
			client.AddTorrent("torrent_files/Atherton - Rivers of Fire.mp3.torrent")
			/*
				hwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("BitTorrent_Client"))
				win.SetWindowLong(hwnd, win.GWL_EXSTYLE, win.GetWindowLong(hwnd, win.GWL_EXSTYLE)|win.WS_EX_LAYERED)*/

		},
		Bind: []interface{}{
			app,
			client,
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
