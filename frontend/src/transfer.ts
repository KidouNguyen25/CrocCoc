export const TRANSFER_UPDATE = 'transfer:update'

export type TransferState =
  | 'IDLE'
  | 'PREPARING'
  | 'WAITING_FOR_PEER'
  | 'TRANSFERRING'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELLED'

export interface CrocInfo {
  available: boolean
  version?: string
  path?: string
  message?: string
}

export interface TransferUpdate {
  transferID: string
  state: TransferState
  currentFile?: string
  bytesTransferred?: number
  bytesTotal?: number
  bytesPerSecond?: number
  progress?: number
  message?: string
  connectionCode?: string
}

export interface TransferClient {
  getCrocInfo(): Promise<CrocInfo>
  startSend(paths: string[]): Promise<string>
  startReceive(code: string, outputDir: string): Promise<string>
  cancelTransfer(transferID: string): Promise<boolean>
  subscribe(handler: (update: TransferUpdate) => void): () => void
}

export function isRunning(state: TransferState): boolean {
  return state === 'PREPARING' || state === 'WAITING_FOR_PEER' || state === 'TRANSFERRING'
}
