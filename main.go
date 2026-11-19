package main

import (
	"embed"
	"log"

	"croccoc/backend/transfer"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := wails.Run(&options.App{
		Title:     "CrocCoc",
		Width:     920,
		Height:    690,
		MinWidth:  480,
		MinHeight: 540,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{newApp(transfer.NewUnavailableService())},
	}); err != nil {
		log.Fatal(err)
	}
}
