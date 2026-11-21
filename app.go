package main

import (
	"context"
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

func (a *App) GetReceiveOffer(transferID string) (transfer.ReceiveOffer, bool) {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return transfer.ReceiveOffer{}, false
	}
	return controller.GetReceiveOffer(transferID)
}

func (a *App) GetTransferSnapshot(transferID string) (transfer.SessionSnapshot, bool) {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return transfer.SessionSnapshot{}, false
	}
	return controller.GetTransferSnapshot(transferID)
}

func (a *App) GetSendCode(transferID string) (string, bool) {
	controller, ok := a.transfers.(transfer.EngineController)
	if !ok {
		return "", false
	}
	return controller.GetSendCode(transferID)
}

func (a *App) setRuntimeContext(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
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
