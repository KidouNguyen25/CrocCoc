import { useEffect, useState, useRef, useMemo } from 'react'
import { getDesktopClient } from './adapters/wailsTransfer.ts'
import { createMockTransferClient } from './mockTransfer.ts'
import { acceptTransferUpdate, isRunning, preserveStartedTransferUpdate } from './transfer.ts'
import type { CrocInfo, TransferClient, TransferUpdate } from './transfer.ts'
import { Header } from './components/Header.tsx'
import { ModeSwitcher } from './components/ModeSwitcher.tsx'
import { SendPanel } from './components/SendPanel.tsx'
import { ReceivePanel } from './components/ReceivePanel.tsx'

export function App() {
  const [mode, setMode] = useState<'send' | 'receive'>('send')
  const [hasDesktopRuntime, setHasDesktopRuntime] = useState(false)
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
  const activeTransferID = useRef<string | undefined>(undefined)
  const awaitingStart = useRef(false)
  const earlyUpdates = useRef(new Map<string, TransferUpdate>())

  // Identify runtime environment and active client
  useEffect(() => {
    const desktop = getDesktopClient()
    setHasDesktopRuntime(Boolean(desktop))
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
      if (!event.transferID) return
      if (awaitingStart.current && !activeTransferID.current) {
        earlyUpdates.current.set(
          event.transferID,
          acceptTransferUpdate(earlyUpdates.current.get(event.transferID), event),
        )
        return
      }
      if (event.transferID !== activeTransferID.current) return
      setUpdate((current) => acceptTransferUpdate(current, event))
      if (event.connectionCode) {
        setConnectionCode(event.connectionCode)
      } else if (event.state !== 'WAITING_FOR_PEER' && event.state !== 'PREPARING') {
        setConnectionCode('')
      }
    })

    return () => {
      unsubscribe()
    }
  }, [useMock, mockClient])

  const state = update?.state ?? 'IDLE'
  const busy = pending || isRunning(state)

  const beginTransfer = async (request: () => Promise<string>, fallbackError: string) => {
    // The native engine can emit a WAITING/FAILED event before Start* resolves.
    // Buffer it by ID and never overwrite it with a synthetic PREPARING state.
    awaitingStart.current = true
    activeTransferID.current = undefined
    earlyUpdates.current.clear()
    setPending(true)
    setAppError('')
    setConnectionCode('')
    setUpdate(undefined)
    try {
      const transferID = await request()
      activeTransferID.current = transferID
      awaitingStart.current = false
      const buffered = earlyUpdates.current.get(transferID)
      earlyUpdates.current.clear()
      setUpdate((current) => {
        const initial = preserveStartedTransferUpdate(current, transferID)
        return buffered ? acceptTransferUpdate(initial, buffered) : initial
      })
      if (buffered?.connectionCode) setConnectionCode(buffered.connectionCode)
      if (!buffered && clientRef.current.getTransferSnapshot) {
        void clientRef.current.getTransferSnapshot(transferID).then((snapshot) => {
          if (!snapshot || activeTransferID.current !== transferID) return
          setUpdate((current) => acceptTransferUpdate(current, snapshot))
        }).catch(() => {})
      }
    } catch (err) {
      setAppError(err instanceof Error ? err.message : fallbackError)
      throw err
    } finally {
      awaitingStart.current = false
      earlyUpdates.current.clear()
      setPending(false)
    }
  }

  const handleStartSend = (paths: string[]) =>
    beginTransfer(() => clientRef.current.startSend(paths), 'Lỗi khi bắt đầu phiên gửi.')

  const handleStartReceive = (code: string, outputDir: string) =>
    beginTransfer(() => clientRef.current.startReceive(code, outputDir), 'Lỗi khi bắt đầu phiên nhận.')

  const handleSelectPaths = (kind: 'sendFiles' | 'sendDirectory' | 'receiveDirectory') => {
    const selectPaths = clientRef.current.selectPaths
    if (!selectPaths) return Promise.reject(new Error('Không có hộp thoại chọn đường dẫn của desktop.'))
    return selectPaths(kind)
  }

  const handleCopySendCode = async () => {
    if (!update?.transferID || !clientRef.current.copySendCode) {
      throw new Error('COPY_UNSAFE: không thể sao chép mã an toàn trên thiết bị này.')
    }
    await clientRef.current.copySendCode(update.transferID)
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
    } else if (!accept) {
      await handleCancel()
    }
  }

  const handleReset = () => {
    activeTransferID.current = undefined
    awaitingStart.current = false
    earlyUpdates.current.clear()
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
        <p className="overline">TRUYỀN TỆP MÃ HÓA CROC PAKE</p>
        <h1 id="title">Chuyển dữ liệu nhanh & an toàn</h1>
        <p className="intro-desc">
          Nội dung tệp được mã hóa đầu cuối theo chuẩn Croc PAKE. Bản MVP truyền qua máy chủ relay trung gian (relay quan sát được metadata mạng và thời điểm, không đọc được nội dung).
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
              onSelectPaths={isDesktop && !useMock ? handleSelectPaths : undefined}
              onCopySendCode={isDesktop && !useMock ? handleCopySendCode : undefined}
              onCancel={handleCancel}
              onReset={handleReset}
            />
          ) : (
            <ReceivePanel
              update={update}
              busy={busy}
              isMock={!isDesktop || useMock}
              onStartReceive={handleStartReceive}
              onSelectReceiveDirectory={isDesktop && !useMock ? () => handleSelectPaths('receiveDirectory') : undefined}
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
          {hasDesktopRuntime && (
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
