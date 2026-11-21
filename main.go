package main

import (
	"context"
	"embed"
	"log"
	"os"
	"strings"

	"croccoc/backend/transfer"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := newApp(transfer.NewUnavailableService())
	app.transfers = transfer.NewEngineService(engineServiceConfig(app.emitTransferUpdate))
	if err := wails.Run(&options.App{
		Title:     "CrocCoc",
		Width:     920,
		Height:    690,
		MinWidth:  480,
		MinHeight: 540,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.setRuntimeContext(ctx)
		},
		Bind: []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}

func engineServiceConfig(eventSink func(transfer.Update)) transfer.EngineServiceConfig {
	workerPath := strings.TrimSpace(os.Getenv("CROCCOC_WORKER_PATH"))
	relayAddress := strings.TrimSpace(os.Getenv("CROCCOC_RELAY_ADDRESS"))
	relayPassword := os.Getenv("CROCCOC_RELAY_PASSWORD")
	_ = os.Unsetenv("CROCCOC_RELAY_PASSWORD")
	return transfer.EngineServiceConfig{
		WorkerPath:    workerPath,
		RelayAddress:  relayAddress,
		RelayPassword: relayPassword,
		EventSink:     eventSink,
	}
}
