export const TRANSFER_UPDATE = 'transfer:update'

export type TransferState =
  | 'IDLE'
  | 'PREPARING'
  | 'WAITING_FOR_PEER'
  | 'AWAITING_ACCEPTANCE'
  | 'TRANSFERRING'
  | 'FINALIZING'
  | 'CANCELLING'
  | 'COMPLETED'
  | 'FAILED'
  | 'CANCELLED'

export interface CrocInfo {
  available: boolean
  version?: string
  path?: string
  message?: string
}

export interface OfferItem {
  path: string
  type: 'file' | 'directory' | string
  sizeBytes?: string
}

export interface ReceiveOffer {
  offerID: string
  items: OfferItem[]
  fileCount: number
  totalBytes: string
  expiresAt: string
}

export interface TransferError {
  code: string
  message: string
}

export interface TransferUpdate {
  transferID: string
  revision?: number
  state: TransferState
  currentFile?: string
  bytesTransferred?: number
  bytesTotal?: number
  bytesPerSecond?: number
  progress?: number
  message?: string
  connectionCode?: string
  receiveOffer?: ReceiveOffer
  error?: TransferError
  partialOutput?: boolean
  cleanupPending?: boolean
}

export interface SessionSnapshot {
  transferID: string
  revision: number
  state: TransferState
  partialOutput?: boolean
  cleanupPending?: boolean
}

export type PathSelectionKind = 'sendFiles' | 'sendDirectory' | 'receiveDirectory'

export interface TransferClient {
  getCrocInfo(): Promise<CrocInfo>
  startSend(paths: string[]): Promise<string>
  startReceive(code: string, outputDir: string): Promise<string>
  cancelTransfer(transferID: string): Promise<boolean>
  decideReceive?(transferID: string, offerID: string, accept: boolean): Promise<boolean>
  subscribe(handler: (update: TransferUpdate) => void): () => void
  selectPaths?(kind: PathSelectionKind): Promise<string[]>
  copySendCode?(transferID: string): Promise<void>
  getTransferSnapshot?(transferID: string): Promise<SessionSnapshot | null>
}

export function preserveStartedTransferUpdate(
  current: TransferUpdate | undefined,
  transferID: string,
): TransferUpdate {
  return current?.transferID === transferID
    ? current
    : { transferID, state: 'PREPARING' }
}

// A delayed or duplicate event must never rewind a live session or its terminal state.
export function acceptTransferUpdate(
  current: TransferUpdate | undefined,
  next: TransferUpdate,
): TransferUpdate {
  if (!current) return next
  if (current.transferID !== next.transferID) return current
  const terminal = ['COMPLETED', 'FAILED', 'CANCELLED']
  if (terminal.includes(current.state)) return current
  if (current.revision !== undefined && (next.revision ?? 0) <= current.revision) return current
  return next
}

export function isRunning(state: TransferState): boolean {
  return (
    state === 'PREPARING' ||
    state === 'WAITING_FOR_PEER' ||
    state === 'AWAITING_ACCEPTANCE' ||
    state === 'TRANSFERRING' ||
    state === 'FINALIZING' ||
    state === 'CANCELLING'
  )
}
