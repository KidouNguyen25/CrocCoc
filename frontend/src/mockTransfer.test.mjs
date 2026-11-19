import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createMockTransferClient, SAMPLE_CODE } from './mockTransfer.ts'

function harness() {
  const pending = new Set()
  const events = []
  const client = createMockTransferClient((callback) => {
    pending.add(callback)
    return () => pending.delete(callback)
  })
  client.subscribe((event) => events.push(event))
  const run = () => {
    for (const task of pending) {
      pending.delete(task)
      task()
    }
  }
  return { client, events, run }
}

test('send emits ordered states, sample code and one transfer ID', async () => {
  const { client, events, run } = harness()
  const transferID = await client.startSend(['ghi chu.txt'])
  await assert.rejects(client.startReceive(SAMPLE_CODE, 'Downloads'))
  run()
  assert.deepEqual(events.map((event) => event.state), [
    'PREPARING', 'WAITING_FOR_PEER', 'TRANSFERRING', 'COMPLETED',
  ])
  assert(events.every((event) => event.transferID === transferID && !('progress' in event)))
  assert.equal(events[1].connectionCode, SAMPLE_CODE)
  assert.notEqual(await client.startSend(['second.txt']), transferID)
})

test('receive validates inputs and cancellation emits no later updates', async () => {
  const { client, events, run } = harness()
  await assert.rejects(client.startSend([]))
  await assert.rejects(client.startReceive('private-code', 'Downloads'), (error) => !error.message.includes('private-code'))
  await assert.rejects(client.startReceive(SAMPLE_CODE, ''))
  const id = await client.startReceive(SAMPLE_CODE, 'Downloads')
  assert.equal(await client.cancelTransfer('other'), false)
  assert.equal(await client.cancelTransfer(id), true)
  run()
  assert.deepEqual(events, [{ transferID: id, state: 'CANCELLED' }])
  assert.equal(await client.cancelTransfer(id), false)
})
