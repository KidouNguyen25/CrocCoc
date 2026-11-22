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
  bytesTransferred?: string | number
  bytesTotal?: string | number
  bytesPerSecond?: number
  progress?: number
  message?: string
  connectionCode?: string
  receiveOffer?: ReceiveOffer
  error?: TransferError
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
  getTransferSnapshot?(transferID: string): Promise<TransferUpdate | null>
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
  const terminal: TransferState[] = ['COMPLETED', 'FAILED', 'CANCELLED']
  if (terminal.includes(current.state)) return current
  if (current.revision !== undefined && (next.revision ?? 0) <= current.revision) return current
  return next
}

// Tracks active transfer identity across consecutive sessions so early native
// events arriving before Start* resolves are never dropped by a prior terminal session.
export interface TransferSessionTracker {
  beginStart(): void
  onEvent(event: TransferUpdate): TransferUpdate | undefined
  resolveStart(transferID: string): { update: TransferUpdate; hadBuffered: boolean }
  reconcileSnapshot(transferID: string, snapshot: TransferUpdate | null): TransferUpdate | undefined
  failStart(): void
  reset(): void
  current(): TransferUpdate | undefined
}

export function createTransferSessionTracker(): TransferSessionTracker {
  let activeID: string | undefined
  let awaitingStart = false
  let latest: TransferUpdate | undefined
  const earlyByID = new Map<string, TransferUpdate>()

  return {
    beginStart() {
      awaitingStart = true
      activeID = undefined
      latest = undefined
      earlyByID.clear()
    },
    onEvent(event) {
      if (!event || !event.transferID) return undefined
      if (awaitingStart && !activeID) {
        earlyByID.set(
          event.transferID,
          acceptTransferUpdate(earlyByID.get(event.transferID), event),
        )
        return undefined
      }
      if (event.transferID !== activeID) return undefined
      const accepted = acceptTransferUpdate(latest, event)
      latest = accepted
      return accepted
    },
    resolveStart(transferID) {
      activeID = transferID
      awaitingStart = false
      const buffered = earlyByID.get(transferID)
      earlyByID.clear()
      const initial = preserveStartedTransferUpdate(undefined, transferID)
      latest = buffered ? acceptTransferUpdate(initial, buffered) : initial
      return { update: latest, hadBuffered: Boolean(buffered) }
    },
    reconcileSnapshot(transferID, snapshot) {
      if (!snapshot || activeID !== transferID || snapshot.transferID !== transferID) {
        return undefined
      }
      const accepted = acceptTransferUpdate(latest, snapshot)
      latest = accepted
      return accepted
    },
    failStart() {
      awaitingStart = false
      earlyByID.clear()
    },
    reset() {
      activeID = undefined
      awaitingStart = false
      latest = undefined
      earlyByID.clear()
    },
    current() {
      return latest
    },
  }
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
