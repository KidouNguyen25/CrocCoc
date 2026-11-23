export const TRANSFER_UPDATE = 'transfer:update'
export const MAX_SNAPSHOT_RETRIES = 3
export const SNAPSHOT_POLL_INTERVAL_MS = 5000
export const SNAPSHOT_LOST_TRACKING_MESSAGE =
  'Mất theo dõi trạng thái phiên truyền sau nhiều lần đồng bộ snapshot thất bại.'

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
  recordSnapshotFailure(
    transferID: string,
    maxRetries?: number,
  ): { failureCount: number; trackingLost: boolean }
  snapshotFailures(): number
  failStart(): void
  reset(): void
  current(): TransferUpdate | undefined
}

export function createTransferSessionTracker(): TransferSessionTracker {
  let activeID: string | undefined
  let awaitingStart = false
  let latest: TransferUpdate | undefined
  let failureCount = 0
  const earlyByID = new Map<string, TransferUpdate>()

  return {
    beginStart() {
      awaitingStart = true
      activeID = undefined
      latest = undefined
      failureCount = 0
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
      // Replayed or out-of-order events do not prove that IPC is healthy.
      // Keep the snapshot failure streak until a newer event is accepted.
      if (accepted === latest) return undefined
      failureCount = 0
      latest = accepted
      return accepted
    },
    resolveStart(transferID) {
      activeID = transferID
      awaitingStart = false
      failureCount = 0
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
      // A stale snapshot with a lower revision than already observed does not prove
      // healthy IPC progress and must not clear the snapshot failure streak.
      if (
        latest &&
        latest.transferID === transferID &&
        latest.revision !== undefined &&
        (snapshot.revision ?? 0) < latest.revision
      ) {
        return undefined
      }
      failureCount = 0
      const accepted = acceptTransferUpdate(latest, snapshot)
      latest = accepted
      return accepted
    },
    recordSnapshotFailure(transferID, maxRetries = MAX_SNAPSHOT_RETRIES) {
      if (activeID !== transferID || !latest || !isRunning(latest.state)) {
        return { failureCount: 0, trackingLost: false }
      }
      failureCount += 1
      return {
        failureCount,
        trackingLost: failureCount >= maxRetries,
      }
    },
    snapshotFailures() {
      return failureCount
    },
    failStart() {
      awaitingStart = false
      failureCount = 0
      earlyByID.clear()
    },
    reset() {
      activeID = undefined
      awaitingStart = false
      latest = undefined
      failureCount = 0
      earlyByID.clear()
    },
    current() {
      return latest
    },
  }
}

export async function syncTransferSnapshot(
  tracker: TransferSessionTracker,
  client: Pick<TransferClient, 'getTransferSnapshot'>,
  transferID: string,
  maxRetries: number = MAX_SNAPSHOT_RETRIES,
): Promise<{ update?: TransferUpdate; trackingLost: boolean; failureCount: number }> {
  if (!client.getTransferSnapshot) {
    return { update: tracker.current(), trackingLost: false, failureCount: 0 }
  }
  const before = tracker.current()
  if (before && before.transferID === transferID && !isRunning(before.state)) {
    return { update: before, trackingLost: false, failureCount: 0 }
  }

  let snapshot: TransferUpdate | null = null
  let callFailed = false
  try {
    snapshot = await client.getTransferSnapshot(transferID)
  } catch {
    callFailed = true
  }

  const after = tracker.current()
  if (!after || after.transferID !== transferID) {
    return { update: undefined, trackingLost: false, failureCount: 0 }
  }
  if (!isRunning(after.state)) {
    return { update: after, trackingLost: false, failureCount: 0 }
  }

  if (!callFailed && snapshot && snapshot.transferID === transferID) {
    const reconciled = tracker.reconcileSnapshot(transferID, snapshot)
    if (reconciled !== undefined) {
      return { update: reconciled, trackingLost: false, failureCount: 0 }
    }
    const currentFailures = tracker.snapshotFailures()
    return {
      update: undefined,
      trackingLost: currentFailures >= maxRetries,
      failureCount: currentFailures,
    }
  }

  const { failureCount, trackingLost } = tracker.recordSnapshotFailure(transferID, maxRetries)
  return { update: undefined, trackingLost, failureCount }
}

export function canToggleMockMode(pending: boolean, state: TransferState): boolean {
  return !pending && !isRunning(state)
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
