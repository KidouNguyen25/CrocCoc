package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
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
		OnBeforeClose: func(ctx context.Context) bool {
			return app.beforeClose(ctx)
		},
		OnShutdown: func(context.Context) {
			app.shutdown()
		},
		Bind: []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}

// resolveWorkerPath only uses an explicit override or the verified bundled path
// alongside the application binary. It never searches PATH or the working dir.
func resolveWorkerPath() string {
	if override := strings.TrimSpace(os.Getenv("CROCCOC_WORKER_PATH")); override != "" {
		return override
	}
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	path := filepath.Join(filepath.Dir(executable), "bin", "croccoc-engine.exe")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

func engineServiceConfig(eventSink func(transfer.Update)) transfer.EngineServiceConfig {
	workerPath := resolveWorkerPath()
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
