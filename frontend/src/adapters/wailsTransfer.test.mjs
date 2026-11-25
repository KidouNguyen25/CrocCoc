import assert from 'node:assert/strict'
import { test } from 'node:test'
import { getDesktopClient, resolveRuntimeMode } from './wailsTransfer.ts'

test('browser preview has no desktop client', () => {
  assert.equal(getDesktopClient({}), undefined)
})

test('desktop adapter preserves binding arguments and transfer event', async () => {
  const calls = []
  let handler
  const host = {
    go: { main: { App: {
      GetCrocInfo: async () => ({ available: true }),
      StartSend: async (paths) => { calls.push(paths); return 'send-id' },
      StartReceive: async (...args) => { calls.push(args); return 'receive-id' },
      CancelTransfer: async (id) => { calls.push(id); return true },
    } } },
    runtime: {
      EventsOn(event, callback) {
        calls.push(event)
        handler = callback
        return () => { handler = undefined }
      },
    },
  }
  const client = getDesktopClient(host)
  assert.deepEqual(await client.getCrocInfo(), { available: true })
  assert.equal(await client.startSend(['file with space.txt']), 'send-id')
  assert.equal(await client.startReceive('sample-code', 'Downloads'), 'receive-id')
  assert.equal(await client.cancelTransfer('receive-id'), true)
  const events = []
  const unsub = client.subscribe((update) => events.push(update))
  const update = { transferID: 'receive-id', state: 'CANCELLED' }
  handler(update)
  unsub()
  assert.equal(handler, undefined)
  assert.deepEqual(events, [update])
  assert.deepEqual(calls, [['file with space.txt'], ['sample-code', 'Downloads'], 'receive-id', 'transfer:update'])
})

test('desktop path selection uses the native Wails binding', async () => {
  const kinds = []
  const host = {
    go: { main: { App: {
      GetCrocInfo: async () => ({ available: true }),
      StartSend: async () => 'id',
      StartReceive: async () => 'id',
      CancelTransfer: async () => true,
      SelectPaths: async (kind) => {
        kinds.push(kind)
        return kind === 'sendFiles' ? ['C:\\data\\report.txt'] : ['C:\\data']
      },
    } } },
    runtime: { EventsOn: () => () => {} },
  }
  const client = getDesktopClient(host)
  assert.deepEqual(await client.selectPaths('sendFiles'), ['C:\\data\\report.txt'])
  assert.deepEqual(await client.selectPaths('sendDirectory'), ['C:\\data'])
  assert.deepEqual(await client.selectPaths('receiveDirectory'), ['C:\\data'])
  assert.deepEqual(kinds, ['sendFiles', 'sendDirectory', 'receiveDirectory'])
})

test('desktop adapter delegates CopySendCode and GetTransferSnapshot', async () => {
  const copied = []
  const host = {
    go: { main: { App: {
      GetCrocInfo: async () => ({ available: true }),
      StartSend: async () => 'id',
      StartReceive: async () => 'id',
      CancelTransfer: async () => true,
      CopySendCode: async (id) => { copied.push(id) },
      GetTransferSnapshot: async (id) => (id === 'send-1' ? { transferID: 'send-1', revision: 2, state: 'WAITING_FOR_PEER' } : null),
    } } },
    runtime: { EventsOn: () => () => {} },
  }
  const client = getDesktopClient(host)
  await client.copySendCode('send-1')
  assert.deepEqual(copied, ['send-1'])
  assert.deepEqual(await client.getTransferSnapshot('send-1'), { transferID: 'send-1', revision: 2, state: 'WAITING_FOR_PEER' })
  assert.equal(await client.getTransferSnapshot('missing'), null)
})

test('desktop adapter fails loudly when required Wails bindings are missing', async () => {
  const host = {
    go: { main: { App: {
      GetCrocInfo: async () => ({ available: true }),
      StartSend: async () => 'id',
      StartReceive: async () => 'id',
      CancelTransfer: async () => true,
    } } },
    runtime: { EventsOn: () => () => {} },
  }
  const client = getDesktopClient(host)
  await assert.rejects(() => client.selectPaths('sendFiles'), /SelectPaths/)
  await assert.rejects(() => client.copySendCode('id'), /CopySendCode/)
  await assert.rejects(() => client.getTransferSnapshot('id'), /GetTransferSnapshot/)
  await assert.rejects(() => client.decideReceive('id', 'offer', true), /DecideReceive/)
})

test('desktop adapter GetCrocInfo preserves relayLabel and strips unexpected fields', async () => {
  const host = {
    go: { main: { App: {
      GetCrocInfo: async () => ({
        available: true,
        version: '11.6.4',
        relayLabel: 'Relay công cộng: croc.schollz.com:9009',
        relayPassword: 'should-never-cross-ipc',
      }),
      StartSend: async () => 'id',
      StartReceive: async () => 'id',
      CancelTransfer: async () => true,
    } } },
    runtime: { EventsOn: () => () => {} },
  }
  const client = getDesktopClient(host)
  const info = await client.getCrocInfo()
  assert.deepEqual(info, {
    available: true,
    version: '11.6.4',
    relayLabel: 'Relay công cộng: croc.schollz.com:9009',
  })
  assert.equal('relayPassword' in info, false)
})

test('release never falls back to mock even when native IPC is missing', () => {
  for (const nativeHostPresent of [false, true]) {
    for (const mockRequested of [false, true]) {
      assert.equal(resolveRuntimeMode(false, nativeHostPresent, false, mockRequested), 'unavailable')
      assert.equal(resolveRuntimeMode(true, nativeHostPresent, false, mockRequested), 'desktop')
    }
  }
})

test('development permits explicit desktop mock and browser preview, not partial Wails IPC', () => {
  assert.equal(resolveRuntimeMode(false, false, true, false), 'mock')
  assert.equal(resolveRuntimeMode(false, true, true, false), 'unavailable')
  assert.equal(resolveRuntimeMode(false, true, true, true), 'unavailable')
  assert.equal(resolveRuntimeMode(true, true, true, false), 'desktop')
  assert.equal(resolveRuntimeMode(true, true, true, true), 'mock')
})
