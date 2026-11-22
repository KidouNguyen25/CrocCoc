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

const singleInstanceUUID = "8f94d7c2-6b3e-4d1a-9c52-7e3a1b4f8c90"

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
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceUUID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.focusWindow()
			},
		},
		OnStartup: func(ctx context.Context) {
			app.setRuntimeContext(ctx)
		},
		OnBeforeClose: func(context.Context) bool {
			app.shutdown()
			return false
		},
		OnShutdown: func(context.Context) {
			app.shutdown()
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
