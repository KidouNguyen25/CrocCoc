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
