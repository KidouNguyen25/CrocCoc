package main

import (
	"context"
	"errors"
	"sync"

	"croccoc/backend/transfer"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App translates Wails binding calls into application-service calls.
type App struct {
	transfers transfer.Service
	mu        sync.RWMutex
	ctx       context.Context
}

func newApp(transfers transfer.Service) *App {
	return &App{transfers: transfers}
}

func (a *App) GetCrocInfo() transfer.CrocInfo {
	return a.transfers.GetCrocInfo()
}

func (a *App) StartSend(paths []string) (string, error) {
	return a.transfers.StartSend(paths)
}

func (a *App) StartReceive(code, outputDir string) (string, error) {
	return a.transfers.StartReceive(code, outputDir)
}

func (a *App) CancelTransfer(transferID string) (bool, error) {
	return a.transfers.CancelTransfer(transferID)
}

func (a *App) DecideReceive(transferID, offerID string, accept bool) (bool, error) {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return false, transfer.ErrNotIntegrated
	}
	return controller.DecideReceive(transferID, offerID, accept)
}

func (a *App) GetReceiveOffer(transferID string) *transfer.ReceiveOffer {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return nil
	}
	offer, found := controller.GetReceiveOffer(transferID)
	if !found {
		return nil
	}
	return &offer
}

func (a *App) GetTransferSnapshot(transferID string) *transfer.SessionSnapshot {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return nil
	}
	snapshot, found := controller.GetTransferSnapshot(transferID)
	if !found {
		return nil
	}
	return &snapshot
}

// CopySendCode writes the active sender code directly to the OS clipboard with
// Windows monitor/history/cloud exclusion markers. It never accepts arbitrary
// text from WebView and fails closed with COPY_UNSAFE when markers fail.
func (a *App) CopySendCode(transferID string) error {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return transfer.ErrNotIntegrated
	}
	code, found := controller.GetSendCode(transferID)
	if !found {
		return transfer.ErrCopyUnsafe
	}
	return transfer.CopySecretToProtectedClipboard(code)
}

// SelectPaths returns absolute paths from native OS dialogs. Browser File objects
// do not reliably expose paths readable by the desktop transfer engine.
func (a *App) SelectPaths(kind string) ([]string, error) {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return nil, errors.New("desktop runtime is not ready")
	}

	switch kind {
	case "sendFiles":
		paths, err := wailsruntime.OpenMultipleFilesDialog(ctx, wailsruntime.OpenDialogOptions{Title: "Chọn tệp để gửi"})
		if paths == nil && err == nil {
			paths = []string{}
		}
		return paths, err
	case "sendDirectory", "receiveDirectory":
		title := "Chọn thư mục để gửi"
		if kind == "receiveDirectory" {
			title = "Chọn thư mục lưu tệp nhận"
		}
		path, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{Title: title})
		if err != nil {
			return nil, err
		}
		if path == "" {
			return []string{}, nil
		}
		return []string{path}, nil
	default:
		return nil, transfer.ErrInvalidInput
	}
}

func (a *App) setRuntimeContext(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
}

func (a *App) focusWindow() {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
}

func (a *App) shutdown() {
	if controller, ok := a.transfers.(transfer.EngineController); ok {
		controller.Shutdown()
	}
}

func (a *App) emitTransferUpdate(update transfer.Update) {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, transfer.UpdateEvent, update)
}
