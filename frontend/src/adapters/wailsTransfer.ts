import { TRANSFER_UPDATE } from '../transfer.ts'
import type { CrocInfo, TransferClient, TransferUpdate } from '../transfer.ts'

interface WailsApp {
  GetCrocInfo(): Promise<CrocInfo>
  StartSend(paths: string[]): Promise<string>
  StartReceive(code: string, outputDir: string): Promise<string>
  CancelTransfer(transferID: string): Promise<boolean>
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
    subscribe: (handler) => eventsOn.call(runtime, TRANSFER_UPDATE, handler),
  }
}
