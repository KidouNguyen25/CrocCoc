import assert from 'node:assert/strict'
import { test } from 'node:test'
import { getDesktopClient } from './wailsTransfer.ts'

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
