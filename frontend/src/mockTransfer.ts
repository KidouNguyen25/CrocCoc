import type { TransferClient, TransferUpdate } from './transfer.ts'

export const SAMPLE_CODE = 'demo-only-sample-code'
type Schedule = (callback: () => void, delay: number) => () => void
type Session = { id: string; stopTimers: Array<() => void> }

const scheduleAfter: Schedule = (callback, delay) => {
  const timer = setTimeout(callback, delay)
  return () => clearTimeout(timer)
}

export function createMockTransferClient(schedule: Schedule = scheduleAfter): TransferClient {
  const listeners = new Set<(update: TransferUpdate) => void>()
  let active: Session | undefined
  let sequence = 0

  const publish = (update: TransferUpdate) => {
    for (const listener of listeners) listener(update)
  }

  const start = async (direction: 'send' | 'receive') => {
    if (active) throw new Error('Đang có một phiên mô phỏng chạy.')
    const id = `mock-${++sequence}`
    const session: Session = { id, stopTimers: [] }
    active = session
    const steps: Array<[number, Omit<TransferUpdate, 'transferID'>]> = [
      [0, { state: 'PREPARING' }],
      [250, { state: 'WAITING_FOR_PEER', ...(direction === 'send' ? { connectionCode: SAMPLE_CODE } : {}) }],
      [1000, { state: 'TRANSFERRING' }],
      [2100, { state: 'COMPLETED' }],
    ]
    for (const [delay, update] of steps) {
      session.stopTimers.push(schedule(() => {
        if (active !== session) return
        if (update.state === 'COMPLETED') active = undefined
        publish({ transferID: id, ...update })
      }, delay))
    }
    return id
  }

  return {
    getCrocInfo: async () => ({ available: false, message: 'Đang dùng dịch vụ mô phỏng.' }),
    startSend: async (paths) => {
      if (paths.length === 0 || paths.some((path) => !path.trim())) {
        throw new Error('Cần ít nhất một đường dẫn mẫu hợp lệ.')
      }
      return start('send')
    },
    startReceive: async (code, outputDir) => {
      if (code.trim() !== SAMPLE_CODE || !outputDir.trim()) {
        throw new Error('Cần mã mô phỏng và thư mục đích mẫu.')
      }
      return start('receive')
    },
    cancelTransfer: async (transferID) => {
      if (!active || active.id !== transferID) return false
      const { stopTimers } = active
      active = undefined
      for (const stop of stopTimers) stop()
      publish({ transferID, state: 'CANCELLED' })
      return true
    },
    subscribe: (listener) => {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
  }
}
