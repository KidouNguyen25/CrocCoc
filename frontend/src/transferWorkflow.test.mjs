import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isRunning } from './transfer.ts'
import { getDesktopClient } from './adapters/wailsTransfer.ts'
import { formatBytes, formatSpeed, formatDuration, calculateETA, STATE_LABELS, STATE_DESCRIPTIONS } from './utils/formatters.ts'

test('isRunning correctly identifies active vs final states', () => {
  // Active states
  assert.equal(isRunning('PREPARING'), true)
  assert.equal(isRunning('WAITING_FOR_PEER'), true)
  assert.equal(isRunning('AWAITING_ACCEPTANCE'), true)
  assert.equal(isRunning('TRANSFERRING'), true)
  assert.equal(isRunning('FINALIZING'), true)
  assert.equal(isRunning('CANCELLING'), true)

  // Inactive / final states
  assert.equal(isRunning('IDLE'), false)
  assert.equal(isRunning('COMPLETED'), false)
  assert.equal(isRunning('FAILED'), false)
  assert.equal(isRunning('CANCELLED'), false)
})

test('desktop adapter delegates DecideReceive properly', async () => {
  let decideArgs = null
  const host = {
    go: {
      main: {
        App: {
          GetCrocInfo: async () => ({ available: true, version: '11.6.4' }),
          StartSend: async (paths) => 'send-123',
          StartReceive: async (code, dir) => 'recv-123',
          CancelTransfer: async (id) => true,
          DecideReceive: async (transferID, offerID, accept) => {
            decideArgs = { transferID, offerID, accept }
            return true
          },
        },
      },
    },
    runtime: {
      EventsOn: () => () => {},
    },
  }

  const client = getDesktopClient(host)
  assert(client !== undefined)
  assert(typeof client.decideReceive === 'function')

  const res = await client.decideReceive('recv-123', 'offer-abc', true)
  assert.equal(res, true)
  assert.deepEqual(decideArgs, {
    transferID: 'recv-123',
    offerID: 'offer-abc',
    accept: true,
  })
})

test('file list calculations and item removal simulate user interaction', () => {
  const initialItems = [
    { id: '1', name: 'report.pdf', path: 'C:/docs/report.pdf', size: 1048576, isDirectory: false },
    { id: '2', name: 'images', path: 'C:/docs/images', size: 2097152, isDirectory: true },
    { id: '3', name: 'notes.txt', path: 'C:/docs/notes.txt', size: 512, isDirectory: false },
  ]

  // Total size initially
  const totalSizeInitial = initialItems.reduce((acc, item) => acc + (item.size ?? 0), 0)
  assert.equal(totalSizeInitial, 1048576 + 2097152 + 512)
  assert.equal(formatBytes(totalSizeInitial), '3.0 MB')

  // Remove item 2 (as required by CC-FR-02)
  const remaining = initialItems.filter((i) => i.id !== '2')
  assert.equal(remaining.length, 2)
  assert.equal(remaining.some((i) => i.id === '2'), false)

  const newTotal = remaining.reduce((acc, item) => acc + (item.size ?? 0), 0)
  assert.equal(newTotal, 1048576 + 512)
  assert.equal(formatBytes(newTotal), '1.0 MB')
})

test('STATE_DESCRIPTIONS contains clear guidance for each state', () => {
  const states = [
    'IDLE', 'PREPARING', 'WAITING_FOR_PEER', 'AWAITING_ACCEPTANCE',
    'TRANSFERRING', 'FINALIZING', 'CANCELLING', 'COMPLETED', 'FAILED', 'CANCELLED'
  ]
  for (const s of states) {
    assert(typeof STATE_DESCRIPTIONS[s] === 'string')
    assert(STATE_DESCRIPTIONS[s].length > 10)
  }
})

test('startup does not overwrite early native events and ignores stale revisions', async () => {
  const { acceptTransferUpdate, preserveStartedTransferUpdate } = await import('./transfer.ts')
  const waiting = { transferID: 'send-1', revision: 2, state: 'WAITING_FOR_PEER', connectionCode: 'secret-code' }
  const started = preserveStartedTransferUpdate(waiting, 'send-1')
  assert.equal(started, waiting)
  assert.deepEqual(preserveStartedTransferUpdate(waiting, 'send-2'), { transferID: 'send-2', state: 'PREPARING' })
  assert.equal(acceptTransferUpdate(waiting, { transferID: 'send-1', revision: 1, state: 'PREPARING' }), waiting)
  assert.equal(acceptTransferUpdate(waiting, { ...waiting, revision: 2, state: 'TRANSFERRING' }), waiting)
  assert.equal(acceptTransferUpdate(waiting, { transferID: 'other', revision: 3, state: 'FAILED' }), waiting)
  const completed = acceptTransferUpdate(waiting, { transferID: 'send-1', revision: 4, state: 'COMPLETED' })
  assert.equal(completed.state, 'COMPLETED')
  assert.equal(acceptTransferUpdate(completed, { transferID: 'send-1', revision: 5, state: 'FAILED' }), completed)
})

test('second consecutive session preserves early WAITING_FOR_PEER and reconciles snapshot', async () => {
  const { createTransferSessionTracker } = await import('./transfer.ts')
  const { computeProgressPercent } = await import('./utils/formatters.ts')
  const tracker = createTransferSessionTracker()

  // Session 1 completes to terminal state.
  tracker.beginStart()
  tracker.resolveStart('send-1')
  const done1 = tracker.onEvent({ transferID: 'send-1', revision: 3, state: 'COMPLETED' })
  assert.equal(done1?.state, 'COMPLETED')

  // Session 2 starts WITHOUT explicit reset; early event arrives before StartSend resolves.
  tracker.beginStart()
  assert.equal(
    tracker.onEvent({ transferID: 'send-2', revision: 2, state: 'WAITING_FOR_PEER', connectionCode: 'second-code' }),
    undefined,
  )
  const started2 = tracker.resolveStart('send-2')
  assert.equal(started2.hadBuffered, true)
  assert.equal(started2.update.state, 'WAITING_FOR_PEER')
  assert.equal(started2.update.connectionCode, 'second-code')

  // Session 3 starts with NO early event; GetTransferSnapshot reconciles code and state.
  tracker.beginStart()
  const started3 = tracker.resolveStart('send-3')
  assert.equal(started3.hadBuffered, false)
  assert.equal(started3.update.state, 'PREPARING')
  const reconciled = tracker.reconcileSnapshot('send-3', {
    transferID: 'send-3',
    revision: 2,
    state: 'WAITING_FOR_PEER',
    connectionCode: 'third-code',
  })
  assert.equal(reconciled?.state, 'WAITING_FOR_PEER')
  assert.equal(reconciled?.connectionCode, 'third-code')

  // Exact BigInt progress calculation from decimal string wire pair.
  assert.equal(computeProgressPercent('5000000000000000', '10000000000000000'), 50)
  assert.equal(computeProgressPercent('0', '0'), undefined)
  assert.equal(computeProgressPercent('150', '100'), undefined)
})

test('syncTransferSnapshot recovers missed FAILED terminal error and enforces bounded retries', async () => {
  const {
    createTransferSessionTracker,
    syncTransferSnapshot,
    MAX_SNAPSHOT_RETRIES,
  } = await import('./transfer.ts')

  const tracker = createTransferSessionTracker()
  tracker.beginStart()
  // Early WAITING_FOR_PEER event was buffered before Start* resolved.
  tracker.onEvent({
    transferID: 'recv-42',
    revision: 2,
    state: 'WAITING_FOR_PEER',
  })
  const started = tracker.resolveStart('recv-42')
  assert.equal(started.hadBuffered, true)
  assert.equal(started.update.state, 'WAITING_FOR_PEER')

  // Simulate two transient snapshot failures followed by recovery on retry #2.
  let attempts = 0
  const flakyClient = {
    getTransferSnapshot: async () => {
      attempts += 1
      if (attempts === 1) throw new Error('IPC timeout')
      if (attempts === 2) return null
      return {
        transferID: 'recv-42',
        revision: 4,
        state: 'FAILED',
        error: {
          code: 'PEER_TIMEOUT',
          message: 'peer không phản hồi trong thời hạn',
        },
      }
    },
  }

  const fail1 = await syncTransferSnapshot(tracker, flakyClient, 'recv-42', MAX_SNAPSHOT_RETRIES)
  assert.equal(fail1.update, undefined)
  assert.equal(fail1.failureCount, 1)
  assert.equal(fail1.trackingLost, false)

  const fail2 = await syncTransferSnapshot(tracker, flakyClient, 'recv-42', MAX_SNAPSHOT_RETRIES)
  assert.equal(fail2.update, undefined)
  assert.equal(fail2.failureCount, 1)
  assert.equal(fail2.trackingLost, true)
  assert.equal(fail2.backendMissing, true)

  // Third call succeeds and recovers the missed FAILED state with its structured error.
  const recovered = await syncTransferSnapshot(tracker, flakyClient, 'recv-42', MAX_SNAPSHOT_RETRIES)
  assert.equal(recovered.failureCount, 0)
  assert.equal(recovered.trackingLost, false)
  assert.equal(recovered.update?.state, 'FAILED')
  assert.deepEqual(recovered.update?.error, {
    code: 'PEER_TIMEOUT',
    message: 'peer không phản hồi trong thời hạn',
  })

  // Next session experiences 3 consecutive snapshot failures -> reports trackingLost = true.
  tracker.beginStart()
  tracker.resolveStart('send-99')
  const deadClient = {
    getTransferSnapshot: async () => { throw new Error('IPC unavailable') },
  }
  const r1 = await syncTransferSnapshot(tracker, deadClient, 'send-99', MAX_SNAPSHOT_RETRIES)
  assert.equal(r1.update, undefined)
  assert.equal(r1.trackingLost, false)
  assert.equal(r1.failureCount, 1)
  const r2 = await syncTransferSnapshot(tracker, deadClient, 'send-99', MAX_SNAPSHOT_RETRIES)
  assert.equal(r2.update, undefined)
  assert.equal(r2.trackingLost, false)
  assert.equal(r2.failureCount, 2)
  const r3 = await syncTransferSnapshot(tracker, deadClient, 'send-99', MAX_SNAPSHOT_RETRIES)
  assert.equal(r3.update, undefined)
  assert.equal(r3.trackingLost, true)
  assert.equal(r3.failureCount, 3)
})

test('an explicit null snapshot means backend no longer tracks the session, not IPC failure', async () => {
  const { createTransferSessionTracker, syncTransferSnapshot } = await import('./transfer.ts')
  const tracker = createTransferSessionTracker()
  tracker.beginStart()
  tracker.resolveStart('gone-1')
  const result = await syncTransferSnapshot(tracker, {
    getTransferSnapshot: async () => null,
  }, 'gone-1')
  assert.equal(result.backendMissing, true)
  assert.equal(result.trackingLost, true)
  assert.equal(result.failureCount, 0)
  assert.equal(tracker.snapshotFailures(), 0)
})

test('backendMissing vs IPC failure distinguishes live event race, recovery, and reset guard', async () => {
  const {
    createTransferSessionTracker,
    syncTransferSnapshot,
    MAX_SNAPSHOT_RETRIES,
  } = await import('./transfer.ts')

  const tracker = createTransferSessionTracker()
  tracker.beginStart()
  tracker.resolveStart('send-race-1')

  // 1. Live native event arriving while a null snapshot RPC is in flight wins over stale null.
  const raceResult = await syncTransferSnapshot(tracker, {
    getTransferSnapshot: async () => {
      tracker.onEvent({
        transferID: 'send-race-1',
        revision: 2,
        state: 'WAITING_FOR_PEER',
        connectionCode: 'live-code',
      })
      return null
    },
  }, 'send-race-1', MAX_SNAPSHOT_RETRIES)
  assert.equal(raceResult.backendMissing, undefined)
  assert.equal(raceResult.trackingLost, false)
  assert.equal(raceResult.failureCount, 0)
  assert.equal(raceResult.update?.state, 'WAITING_FOR_PEER')
  assert.equal(raceResult.update?.connectionCode, 'live-code')

  // 2. One IPC error increments failureCount without setting backendMissing;
  // a subsequent explicit null snapshot sets backendMissing without incrementing failureCount.
  const ipcFail = await syncTransferSnapshot(tracker, {
    getTransferSnapshot: async () => {
      throw new Error('IPC bridge error')
    },
  }, 'send-race-1', MAX_SNAPSHOT_RETRIES)
  assert.equal(ipcFail.backendMissing, undefined)
  assert.equal(ipcFail.trackingLost, false)
  assert.equal(ipcFail.failureCount, 1)

  const missingRes = await syncTransferSnapshot(tracker, {
    getTransferSnapshot: async () => null,
  }, 'send-race-1', MAX_SNAPSHOT_RETRIES)
  assert.equal(missingRes.backendMissing, true)
  assert.equal(missingRes.trackingLost, true)
  assert.equal(missingRes.failureCount, 1)
  assert.equal(tracker.snapshotFailures(), 1)

  // 3. Reset guard: if tracker is reset while a snapshot probe is in flight,
  // the late probe response is ignored and cannot mark trackingLost or backendMissing.
  tracker.beginStart()
  tracker.resolveStart('send-reset-guard')
  const afterReset = await syncTransferSnapshot(tracker, {
    getTransferSnapshot: async () => {
      tracker.reset()
      return null
    },
  }, 'send-reset-guard', MAX_SNAPSHOT_RETRIES)
  assert.equal(afterReset.update, undefined)
  assert.equal(afterReset.backendMissing, undefined)
  assert.equal(afterReset.trackingLost, false)
  assert.equal(afterReset.failureCount, 0)
  assert.equal(tracker.current(), undefined)
})

test('stale or duplicate native events cannot erase snapshot failure streak', async () => {
  const { createTransferSessionTracker, syncTransferSnapshot } = await import('./transfer.ts')
  const tracker = createTransferSessionTracker()
  tracker.beginStart()
  tracker.resolveStart('send-1')
  const waiting = { transferID: 'send-1', revision: 3, state: 'WAITING_FOR_PEER' }
  assert.deepEqual(tracker.onEvent(waiting), waiting)

  const unavailable = { getTransferSnapshot: async () => { throw new Error('IPC unavailable') } }
  assert.equal((await syncTransferSnapshot(tracker, unavailable, 'send-1')).failureCount, 1)

  // An out-of-order event and a duplicate are not evidence that IPC recovered.
  assert.equal(tracker.onEvent({ ...waiting, revision: 2, state: 'PREPARING' }), undefined)
  assert.equal(tracker.onEvent(waiting), undefined)
  assert.equal(tracker.snapshotFailures(), 1)

  assert.equal((await syncTransferSnapshot(tracker, unavailable, 'send-1')).failureCount, 2)
  assert.equal(tracker.onEvent({ transferID: 'other', revision: 99, state: 'FAILED' }), undefined)
  const lost = await syncTransferSnapshot(tracker, unavailable, 'send-1')
  assert.equal(lost.trackingLost, true)
  assert.equal(lost.failureCount, 3)

  // A new legitimate revision recovers tracking.
  assert.deepEqual(
    tracker.onEvent({ transferID: 'send-1', revision: 4, state: 'TRANSFERRING' }),
    { transferID: 'send-1', revision: 4, state: 'TRANSFERRING' },
  )
  assert.equal(tracker.snapshotFailures(), 0)
})

test('reconcileSnapshot rejects older revision without resetting failure streak and recovers on equal or newer snapshot', async () => {
  const { createTransferSessionTracker, syncTransferSnapshot, MAX_SNAPSHOT_RETRIES } = await import('./transfer.ts')
  const tracker = createTransferSessionTracker()
  tracker.beginStart()
  tracker.resolveStart('send-77')
  const waiting = { transferID: 'send-77', revision: 3, state: 'WAITING_FOR_PEER' }
  assert.deepEqual(tracker.onEvent(waiting), waiting)

  const deadClient = { getTransferSnapshot: async () => { throw new Error('IPC unavailable') } }
  assert.equal((await syncTransferSnapshot(tracker, deadClient, 'send-77')).failureCount, 1)
  assert.equal((await syncTransferSnapshot(tracker, deadClient, 'send-77')).failureCount, 2)
  const lost = await syncTransferSnapshot(tracker, deadClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  assert.equal(lost.trackingLost, true)
  assert.equal(lost.failureCount, 3)

  // Stale snapshot (revision 2 < 3) must be ignored and keep failureCount at 3.
  const staleClient = {
    getTransferSnapshot: async () => ({
      transferID: 'send-77',
      revision: 2,
      state: 'PREPARING',
    }),
  }
  const staleRes = await syncTransferSnapshot(tracker, staleClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  assert.equal(staleRes.update, undefined)
  assert.equal(staleRes.trackingLost, true)
  assert.equal(staleRes.failureCount, 3)
  assert.equal(tracker.snapshotFailures(), 3)

  // Same revision (3 === 3) proves IPC is alive again and resets failure streak.
  const sameRevClient = {
    getTransferSnapshot: async () => waiting,
  }
  const sameRes = await syncTransferSnapshot(tracker, sameRevClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  assert.deepEqual(sameRes.update, waiting)
  assert.equal(sameRes.trackingLost, false)
  assert.equal(sameRes.failureCount, 0)
  assert.equal(tracker.snapshotFailures(), 0)

  // Higher terminal revision after another failure streak recovers terminal state.
  await syncTransferSnapshot(tracker, deadClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  await syncTransferSnapshot(tracker, deadClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  await syncTransferSnapshot(tracker, deadClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  assert.equal(tracker.snapshotFailures(), 3)

  const terminalClient = {
    getTransferSnapshot: async () => ({
      transferID: 'send-77',
      revision: 5,
      state: 'COMPLETED',
    }),
  }
  const terminalRes = await syncTransferSnapshot(tracker, terminalClient, 'send-77', MAX_SNAPSHOT_RETRIES)
  assert.equal(terminalRes.update?.state, 'COMPLETED')
  assert.equal(terminalRes.trackingLost, false)
  assert.equal(terminalRes.failureCount, 0)
})

test('canToggleMockMode blocks switching engine while pending or in any running state', async () => {
  const { canToggleMockMode } = await import('./transfer.ts')
  // Blocked while Start* is pending even if state is still IDLE.
  assert.equal(canToggleMockMode(true, 'IDLE'), false)
  // Blocked in all running states.
  for (const state of [
    'PREPARING',
    'WAITING_FOR_PEER',
    'AWAITING_ACCEPTANCE',
    'TRANSFERRING',
    'FINALIZING',
    'CANCELLING',
  ]) {
    assert.equal(canToggleMockMode(false, state), false)
  }
  // Permitted only when not pending and in IDLE or terminal states.
  for (const state of ['IDLE', 'COMPLETED', 'FAILED', 'CANCELLED']) {
    assert.equal(canToggleMockMode(false, state), true)
  }
})
