import { TRANSFER_UPDATE } from '../transfer.ts'
import type { CrocInfo, PathSelectionKind, SessionSnapshot, TransferClient, TransferUpdate } from '../transfer.ts'

interface WailsApp {
  GetCrocInfo(): Promise<CrocInfo>
  StartSend(paths: string[]): Promise<string>
  StartReceive(code: string, outputDir: string): Promise<string>
  CancelTransfer(transferID: string): Promise<boolean>
  DecideReceive?(transferID: string, offerID: string, accept: boolean): Promise<boolean>
  SelectPaths?(kind: PathSelectionKind): Promise<string[]>
  CopySendCode?(transferID: string): Promise<void>
  GetTransferSnapshot?(transferID: string): Promise<SessionSnapshot | null>
}

interface WailsHost {
  go?: { main?: { App?: WailsApp } }
  runtime?: { EventsOn?: (event: string, handler: (update: TransferUpdate) => void) => () => void }
}

export function getDesktopClient(host: WailsHost = window as Window & WailsHost): TransferClient | undefined {
  const binding = host.go?.main?.App
  const runtime = host.runtime
  const eventsOn = runtime?.EventsOn
  if (!binding || !eventsOn) return undefined

  return {
    getCrocInfo: () => binding.GetCrocInfo(),
    startSend: (paths) => binding.StartSend(paths),
    startReceive: (code, outputDir) => binding.StartReceive(code, outputDir),
    cancelTransfer: (id) => binding.CancelTransfer(id),
    decideReceive: binding.DecideReceive
      ? (transferID, offerID, accept) => binding.DecideReceive!(transferID, offerID, accept)
      : undefined,
    subscribe: (handler) => eventsOn.call(runtime, TRANSFER_UPDATE, handler),
    selectPaths: binding.SelectPaths ? (kind) => binding.SelectPaths!(kind) : undefined,
    copySendCode: binding.CopySendCode ? (transferID) => binding.CopySendCode!(transferID) : undefined,
    getTransferSnapshot: binding.GetTransferSnapshot
      ? (transferID) => binding.GetTransferSnapshot!(transferID)
      : undefined,
  }
}
