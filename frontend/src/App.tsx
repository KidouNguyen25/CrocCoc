import { useEffect, useState, useRef, useMemo } from 'react'
import { getDesktopClient } from './adapters/wailsTransfer.ts'
import { createMockTransferClient } from './mockTransfer.ts'
import { isRunning } from './transfer.ts'
import type { CrocInfo, TransferClient, TransferUpdate } from './transfer.ts'
import { Header } from './components/Header.tsx'
import { ModeSwitcher } from './components/ModeSwitcher.tsx'
import { SendPanel } from './components/SendPanel.tsx'
import { ReceivePanel } from './components/ReceivePanel.tsx'

export function App() {
  const [mode, setMode] = useState<'send' | 'receive'>('send')
  const [isDesktop, setIsDesktop] = useState(false)
  const [useMock, setUseMock] = useState(false)
  const [crocInfo, setCrocInfo] = useState<CrocInfo>()
  const [hostTitle, setHostTitle] = useState('Đang khởi tạo...')
  const [update, setUpdate] = useState<TransferUpdate>()
  const [connectionCode, setConnectionCode] = useState('')
  const [pending, setPending] = useState(false)
  const [appError, setAppError] = useState('')

  // Create persistent mock client instance for browser preview
  const mockClient = useMemo(() => createMockTransferClient(), [])
  const clientRef = useRef<TransferClient>(mockClient)

  // Identify runtime environment and active client
  useEffect(() => {
    const desktop = getDesktopClient()
    if (desktop && !useMock) {
      setIsDesktop(true)
      clientRef.current = desktop
      setHostTitle('Ứng dụng desktop (Wails IPC)')
      desktop.getCrocInfo().then(
        (info) => {
          setCrocInfo(info)
          if (!info.available) {
            setHostTitle('Ứng dụng desktop · Chưa phát hiện Croc')
          }
        },
        () => {
          setCrocInfo({ available: false, message: 'Không thể kết nối Croc engine.' })
          setHostTitle('Ứng dụng desktop · Lỗi kiểm tra Croc')
        },
      )
    } else {
      setIsDesktop(false)
      clientRef.current = mockClient
      setHostTitle(useMock ? 'Chế độ mô phỏng thủ công' : 'Bản xem trước trình duyệt (Mock)')
      mockClient.getCrocInfo().then((info) => setCrocInfo(info))
    }

    // Subscribe to transfer events
    const unsubscribe = clientRef.current.subscribe((event) => {
      setUpdate(event)
      if (event.connectionCode) {
        setConnectionCode(event.connectionCode)
      }
      if (event.state === 'CANCELLED' || event.state === 'FAILED') {
        // Keep connection code if already generated, unless cancelled before generation
      }
    })

    return () => {
      unsubscribe()
    }
  }, [useMock, mockClient])

  const state = update?.state ?? 'IDLE'
  const busy = pending || isRunning(state)

  const handleStartSend = async (paths: string[]) => {
    setPending(true)
    setAppError('')
    setConnectionCode('')
    try {
      const transferID = await clientRef.current.startSend(paths)
      setUpdate({ transferID, state: 'PREPARING' })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Lỗi khi bắt đầu phiên gửi.'
      setAppError(message)
      throw err
    } finally {
      setPending(false)
    }
  }

  const handleStartReceive = async (code: string, outputDir: string) => {
    setPending(true)
    setAppError('')
    setConnectionCode(code)
    try {
      const transferID = await clientRef.current.startReceive(code, outputDir)
      setUpdate({ transferID, state: 'PREPARING' })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Lỗi khi bắt đầu phiên nhận.'
      setAppError(message)
      throw err
    } finally {
      setPending(false)
    }
  }

  const handleCancel = async () => {
    if (!update?.transferID) return
    try {
      await clientRef.current.cancelTransfer(update.transferID)
    } catch (err) {
      setAppError(err instanceof Error ? err.message : 'Không thể hủy phiên.')
    }
  }

  const handleDecideReceive = async (accept: boolean) => {
    if (!update?.transferID || !update.receiveOffer) return
    const offerID = update.receiveOffer.offerID
    if (clientRef.current.decideReceive) {
      try {
        await clientRef.current.decideReceive(update.transferID, offerID, accept)
      } catch (err) {
        setAppError(err instanceof Error ? err.message : 'Không thể gửi quyết định xác nhận.')
      }
    } else {
      // In mock without explicit decideReceive, accept continues or cancel stops
      if (!accept) {
        await handleCancel()
      }
    }
  }

  const handleReset = () => {
    setUpdate(undefined)
    setConnectionCode('')
    setAppError('')
  }

  const handleModeChange = (newMode: 'send' | 'receive') => {
    if (busy) return
    setMode(newMode)
    handleReset()
  }

  return (
    <main className="shell">
      <Header hostTitle={hostTitle} crocInfo={crocInfo} isDesktop={isDesktop} />

      <section className="intro" aria-labelledby="title">
        <p className="overline">TRUYỀN TỆP NGANG HÀNG CROC</p>
        <h1 id="title">Chuyển dữ liệu nhanh & an toàn</h1>
        <p className="intro-desc">
          Truyền tệp và thư mục trực tiếp giữa 2 máy tính mà không cần tải lên máy chủ trung gian.
        </p>
      </section>

      {/* Main interaction panel */}
      <section className="panel" aria-label="Khu vực truyền tệp">
        <ModeSwitcher mode={mode} onModeChange={handleModeChange} disabled={busy} />

        <div className="panel-body">
          {appError && (
            <div className="error-alert top-alert" role="alert">
              <span className="error-icon">⚠</span>
              <span>{appError}</span>
              <button
                type="button"
                className="btn-close-alert"
                onClick={() => setAppError('')}
                aria-label="Đóng thông báo lỗi"
              >
                ✕
              </button>
            </div>
          )}

          {mode === 'send' ? (
            <SendPanel
              update={update}
              connectionCode={connectionCode}
              busy={busy}
              isMock={!isDesktop || useMock}
              onStartSend={handleStartSend}
              onCancel={handleCancel}
              onReset={handleReset}
            />
          ) : (
            <ReceivePanel
              update={update}
              busy={busy}
              isMock={!isDesktop || useMock}
              onStartReceive={handleStartReceive}
              onDecideReceive={handleDecideReceive}
              onCancel={handleCancel}
              onReset={handleReset}
            />
          )}
        </div>
      </section>

      {/* Footer & Mode switcher for development / demo */}
      <footer className="footer">
        <div className="footer-info">
          <span>CrocCoc Desktop v0.1.0 • Chuẩn truyền Croc PAKE</span>
          {isDesktop && (
            <button
              type="button"
              className="btn-toggle-mock"
              onClick={() => {
                setUseMock(!useMock)
                handleReset()
              }}
            >
              {useMock ? 'Chuyển sang Engine thật' : 'Chuyển sang Chế độ mô phỏng'}
            </button>
          )}
        </div>
      </footer>
    </main>
  )
}
