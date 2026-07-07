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

	err := wails.Run(&options.App{
		Title:            "Задачи",
		Width:            1280,
		Height:           840,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.Startup,
		Bind:             []any{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
