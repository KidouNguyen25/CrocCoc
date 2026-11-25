import assert from 'node:assert/strict'
import { test } from 'node:test'
import { computeProgressPercent, formatBytes, formatSpeed, formatDuration, calculateETA, STATE_LABELS } from './formatters.ts'

test('formatBytes formats various sizes correctly', () => {
  assert.equal(formatBytes(undefined), 'Chưa rõ dung lượng')
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(-10), '0 B')
  assert.equal(formatBytes(500), '500 B')
  assert.equal(formatBytes(1024), '1 KB')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes(1048576), '1 MB')
  assert.equal(formatBytes(1073741824), '1 GB')
  assert.equal(formatBytes('2097152'), '2 MB')
})

test('formatSpeed formats transfer rate correctly', () => {
  assert.equal(formatSpeed(undefined), '')
  assert.equal(formatSpeed(0), '0 B/s')
  assert.equal(formatSpeed(1024), '1 KB/s')
  assert.equal(formatSpeed(2500000), '2.4 MB/s')
})

test('formatDuration formats remaining time correctly', () => {
  assert.equal(formatDuration(undefined), '')
  assert.equal(formatDuration(15), '15s')
  assert.equal(formatDuration(60), '1m')
  assert.equal(formatDuration(90), '1m 30s')
  assert.equal(formatDuration(3665), '1h 1m')
})

test('calculateETA estimates remaining time based on progress and rate', () => {
  assert.equal(calculateETA(500, 1000, 100), '5s')
  assert.equal(calculateETA(1000, 1000, 100), '')
  assert.equal(calculateETA(0, 1000, 0), '')
})

test('STATE_LABELS covers all TransferState states', () => {
  const states = [
    'IDLE', 'PREPARING', 'WAITING_FOR_PEER', 'AWAITING_ACCEPTANCE',
    'TRANSFERRING', 'FINALIZING', 'CANCELLING', 'COMPLETED', 'FAILED', 'CANCELLED'
  ]
  for (const s of states) {
    assert(typeof STATE_LABELS[s] === 'string' && STATE_LABELS[s].length > 0)
  }
})

test('wire progress is exact and fails closed for invalid or overflowing values', () => {
  assert.equal(computeProgressPercent('0', '123'), 0)
  assert.equal(computeProgressPercent('9007199254740993', '18014398509481986'), 50)
  assert.equal(computeProgressPercent('9223372036854775807', '9223372036854775807'), 100)
  for (const value of [null, undefined, '', '-1', '1.1', '1e3', NaN, Infinity, -1, 0.5, 9007199254740992, '9223372036854775808', '9'.repeat(1000)]) {
    assert.equal(computeProgressPercent(value, '9223372036854775807'), undefined)
    assert.equal(computeProgressPercent('0', value), undefined)
  }
  assert.equal(computeProgressPercent('1', '0'), undefined)
  assert.equal(computeProgressPercent('2', '1'), undefined)
})
