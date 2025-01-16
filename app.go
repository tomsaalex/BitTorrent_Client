package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/tomsaalex/BitTorrent_Client/backend/torrentclient"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	/*
		hwnd := win.FindWindow(nil, syscall.StringToUTF16Ptr("BitTorrent_Client"))
		win.SetWindowLong(hwnd, win.GWL_EXSTYLE, win.GetWindowLong(hwnd, win.GWL_EXSTYLE)|win.WS_EX_LAYERED)*/

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	testTorrentClient := torrentclient.NewTorrentClient()

	testTorrentClient.AddTorrent("torrent_files/multi-file-torrent-test.torrent")
}
