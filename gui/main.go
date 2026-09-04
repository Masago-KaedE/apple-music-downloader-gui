package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:            "Apple Music Downloader",
		Width:            1180,
		Height:           780,
		MinWidth:         940,
		MinHeight:        650,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 12, G: 17, B: 29, A: 1},
		Windows: &windows.Options{
			Theme:                windows.SystemDefault,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	}); err != nil {
		log.Fatal(err)
	}
}
