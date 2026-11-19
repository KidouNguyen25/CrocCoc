import { useEffect, useState } from 'react'
import { getDesktopClient } from './adapters/wailsTransfer'
import { createMockTransferClient, SAMPLE_CODE } from './mockTransfer'
import { isRunning } from './transfer'
import type { TransferState, TransferUpdate } from './transfer'

const mock = createMockTransferClient()
const stateLabel: Record<TransferState, string> = {
  IDLE: 'Sẵn sàng',
  PREPARING: 'Đang chuẩn bị',
  WAITING_FOR_PEER: 'Đang chờ thiết bị bên kia',
  TRANSFERRING: 'Đang mô phỏng truyền tệp',
  COMPLETED: 'Mô phỏng hoàn tất',
  FAILED: 'Mô phỏng thất bại',
  CANCELLED: 'Đã hủy mô phỏng',
}

export function App() {
  const [mode, setMode] = useState<'send' | 'receive'>('send')
  const [host, setHost] = useState('Bản xem trước trên web')
  const [update, setUpdate] = useState<TransferUpdate>()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [sampleCode, setSampleCode] = useState('')
  const state = update?.state ?? 'IDLE'
  const busy = pending || isRunning(state)

  useEffect(() => {
    const desktop = getDesktopClient()
    if (desktop) {
      setHost('Ứng dụng desktop')
      void desktop.getCrocInfo().then(
        (info) => setHost(info.available ? 'Ứng dụng desktop' : 'Ứng dụng desktop · Chưa kết nối Croc'),
        () => setHost('Ứng dụng desktop · Không đọc được trạng thái Croc'),
      )
    }
    return mock.subscribe((event) => {
      setUpdate(event)
      if (event.connectionCode) setSampleCode(event.connectionCode)
      if (event.state === 'CANCELLED' || event.state === 'FAILED') setSampleCode('')
    })
  }, [])

  const simulate = async () => {
    setPending(true)
    setSampleCode('')
    setError('')
    try {
      const transferID = mode === 'send'
        ? await mock.startSend(['tai-lieu-mau.pdf'])
        : await mock.startReceive(SAMPLE_CODE, 'Downloads')
      setUpdate({ transferID, state: 'PREPARING' })
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Không chạy được mô phỏng.')
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="shell">
      <header className="masthead">
        <div className="wordmark">
          <span className="mark" aria-hidden="true">↔</span>
          <span>CrocCoc</span>
        </div>
        <span className="tag">Bản khởi tạo</span>
      </header>

      <section className="intro" aria-labelledby="title">
        <p className="overline">CROC TRÊN DESKTOP</p>
        <h1 id="title">Gửi và nhận tệp</h1>
        <p>{host}</p>
      </section>

      <section className="panel" aria-label="Mô phỏng luồng truyền tệp">
        <div className="switcher" role="group" aria-label="Luồng mô phỏng">
          <button type="button" aria-pressed={mode === 'send'} disabled={busy} onClick={() => { setMode('send'); setUpdate(undefined); setSampleCode(''); setError('') }}>↑ Gửi</button>
          <button type="button" aria-pressed={mode === 'receive'} disabled={busy} onClick={() => { setMode('receive'); setUpdate(undefined); setSampleCode(''); setError('') }}>↓ Nhận</button>
        </div>
        <div className="details">
          <h2>{mode === 'send' ? 'Gửi tệp mẫu' : 'Nhận tệp mẫu'}</h2>
          <p>{mode === 'send' ? 'Máy bên kia dùng mã kết nối để nhận tệp.' : 'Dùng mã mẫu để xem trạng thái của luồng nhận.'}</p>
          <p className="placeholder">{mode === 'send' ? 'Tệp mẫu · tai-lieu-mau.pdf' : 'Thư mục mô phỏng · Downloads'}</p>
          <div className="controls">
            <button type="button" className="primary" disabled={busy} onClick={() => void simulate()}>
              {mode === 'send' ? 'Mô phỏng gửi' : 'Mô phỏng nhận'}
            </button>
            {isRunning(state) && update && (
              <button type="button" className="cancel" onClick={() => void mock.cancelTransfer(update.transferID)}>Hủy mô phỏng</button>
            )}
          </div>
          <p className="status" role="status"><span aria-hidden="true" />{stateLabel[state]}</p>
          {sampleCode && <p className="sample">Mã giả lập <code>{sampleCode}</code></p>}
          {error && <p role="alert">{error}</p>}
        </div>
      </section>
      <footer>Chế độ mô phỏng. Không gửi, nhận hay ghi tệp thật.</footer>
    </main>
  )
}
