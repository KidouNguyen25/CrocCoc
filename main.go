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

// bundledWorkerRelPath là vị trí worker Croc đã ghim so với CrocCoc.exe trong gói phát hành.
var bundledWorkerRelPath = filepath.Join("bin", "croccoc-engine.exe")

// resolveWorkerPath ưu tiên CROCCOC_WORKER_PATH (môi trường phát triển), nếu không có
// thì dùng worker đi kèm cạnh file thực thi. Trả về chuỗi rỗng khi không tìm thấy;
// backend chỉ báo Croc khả dụng khi worker khớp SHA-256 đã ghim, nên đường dẫn sai không gây truyền giả.
func resolveWorkerPath() string {
	if override := strings.TrimSpace(os.Getenv("CROCCOC_WORKER_PATH")); override != "" {
		return override
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), bundledWorkerRelPath)
	if info, err := os.Stat(candidate); err != nil || info.IsDir() {
		return ""
	}
	return candidate
}
