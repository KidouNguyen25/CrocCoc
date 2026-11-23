import { TRANSFER_UPDATE } from '../transfer.ts'
import type { CrocInfo, PathSelectionKind, TransferClient, TransferUpdate } from '../transfer.ts'

interface WailsApp {
  GetCrocInfo(): Promise<CrocInfo>
  StartSend(paths: string[]): Promise<string>
  StartReceive(code: string, outputDir: string): Promise<string>
  CancelTransfer(transferID: string): Promise<boolean>
  DecideReceive?(transferID: string, offerID: string, accept: boolean): Promise<boolean>
  SelectPaths?(kind: PathSelectionKind): Promise<string[]>
  CopySendCode?(transferID: string): Promise<void>
  GetTransferSnapshot?(transferID: string): Promise<TransferUpdate | null>
}

interface WailsHost {
  go?: { main?: { App?: WailsApp } }
  runtime?: { EventsOn?: (event: string, handler: (update: TransferUpdate) => void) => () => void }
}

function missingBinding(name: string): Promise<never> {
  return Promise.reject(new Error(`ENGINE_UNAVAILABLE: thiếu Wails binding ${name}.`))
}

export function getDesktopClient(host: WailsHost = window as Window & WailsHost): TransferClient | undefined {
  const binding = host.go?.main?.App
  const runtime = host.runtime
  const eventsOn = runtime?.EventsOn
  if (!binding || !eventsOn) return undefined

  return {
    getCrocInfo: async () => {
      const info = await binding.GetCrocInfo()
      return {
        available: Boolean(info?.available),
        ...(info?.version ? { version: info.version } : {}),
        ...(info?.relayLabel ? { relayLabel: info.relayLabel } : {}),
        ...(info?.message ? { message: info.message } : {}),
      }
    },
    startSend: (paths) => binding.StartSend(paths),
    startReceive: (code, outputDir) => binding.StartReceive(code, outputDir),
    cancelTransfer: (id) => binding.CancelTransfer(id),
    decideReceive: (transferID, offerID, accept) =>
      binding.DecideReceive ? binding.DecideReceive(transferID, offerID, accept) : missingBinding('DecideReceive'),
    subscribe: (handler) => eventsOn.call(runtime, TRANSFER_UPDATE, handler),
    selectPaths: (kind) =>
      binding.SelectPaths ? binding.SelectPaths(kind) : missingBinding('SelectPaths'),
    copySendCode: (transferID) =>
      binding.CopySendCode ? binding.CopySendCode(transferID) : missingBinding('CopySendCode'),
    getTransferSnapshot: (transferID) =>
      binding.GetTransferSnapshot
        ? binding.GetTransferSnapshot(transferID)
        : missingBinding('GetTransferSnapshot'),
  }
}
