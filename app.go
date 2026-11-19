package main

import "croccoc/backend/transfer"

// App translates Wails binding calls into application-service calls.
type App struct {
	transfers transfer.Service
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
