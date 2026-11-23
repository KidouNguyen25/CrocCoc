import { useEffect, useState, useRef, useMemo } from 'react'
import { getDesktopClient } from './adapters/wailsTransfer.ts'
import { createMockTransferClient } from './mockTransfer.ts'
import {
  canToggleMockMode,
  createTransferSessionTracker,
  isRunning,
  syncTransferSnapshot,
  MAX_SNAPSHOT_RETRIES,
  SNAPSHOT_POLL_INTERVAL_MS,
  SNAPSHOT_LOST_TRACKING_MESSAGE,
} from './transfer.ts'
import type { CrocInfo, TransferClient, TransferUpdate } from './transfer.ts'
import { Header } from './components/Header.tsx'
import { ModeSwitcher } from './components/ModeSwitcher.tsx'
import { SendPanel } from './components/SendPanel.tsx'
import { ReceivePanel } from './components/ReceivePanel.tsx'

type SnapshotProbeResult = { lost: boolean; backendMissing: boolean }

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
  const [trackingLost, setTrackingLost] = useState(false)
  const [backendMissing, setBackendMissing] = useState(false)
  const [trackingError, setTrackingError] = useState('')
  const [forceResetAvailable, setForceResetAvailable] = useState(false)
  const [pollRecoveryToken, setPollRecoveryToken] = useState(0)
  const [appError, setAppError] = useState('')

  // Create persistent mock client instance for browser preview
  const mockClient = useMemo(() => createMockTransferClient(), [])
  const clientRef = useRef<TransferClient>(mockClient)
  const trackerRef = useRef(createTransferSessionTracker())
  const snapshotInFlightRef = useRef<{ transferID: string; promise: Promise<SnapshotProbeResult> } | null>(null)

  const applyTrackedUpdate = (next: TransferUpdate | undefined) => {
    if (!next) return
    setUpdate(next)
    setTrackingLost(false)
    setBackendMissing(false)
    setTrackingError('')
    setForceResetAvailable(false)
    setAppError((prev) => (prev === SNAPSHOT_LOST_TRACKING_MESSAGE ? '' : prev))
    if (next.connectionCode) {
      setConnectionCode(next.connectionCode)
    } else if (next.state !== 'WAITING_FOR_PEER' && next.state !== 'PREPARING') {
      setConnectionCode('')
    }
  }

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
      const trackingWasLost = trackerRef.current.snapshotFailures() >= MAX_SNAPSHOT_RETRIES
      const accepted = trackerRef.current.onEvent(event)
      if (accepted && trackingWasLost) {
        // A fresh native event restores tracking; resume polling for a missed terminal event.
        setPollRecoveryToken((token) => token + 1)
      }
      applyTrackedUpdate(accepted)
    })

    return () => {
      unsubscribe()
    }
  }, [useMock, mockClient])

  const state = update?.state ?? 'IDLE'
  const busy = !canToggleMockMode(pending, state)
  const activeTransferID = update?.transferID
  const activeRunning = Boolean(activeTransferID && isRunning(state))
  const displayedError = trackingError || appError

  const probeSnapshot = (
    transferID: string,
    allowWhenLost = false,
  ): Promise<SnapshotProbeResult> => {
    const noChange: SnapshotProbeResult = { lost: false, backendMissing: false }
    const inFlight = snapshotInFlightRef.current
    if (inFlight) {
      // A cancel request must await the already-running focus/timer probe
      // rather than using an old failure counter to enable forced reset.
      return inFlight.transferID === transferID ? inFlight.promise : Promise.resolve(noChange)
    }
    if (!clientRef.current.getTransferSnapshot) return Promise.resolve(noChange)
    const wasLost = trackerRef.current.snapshotFailures() >= MAX_SNAPSHOT_RETRIES
    if (wasLost && !allowWhenLost) return Promise.resolve(noChange)

    const promise = (async (): Promise<SnapshotProbeResult> => {
      const result = await syncTransferSnapshot(
        trackerRef.current,
        clientRef.current,
        transferID,
        MAX_SNAPSHOT_RETRIES,
      )
      if (result.backendMissing) {
        setTrackingLost(true)
        setBackendMissing(true)
        setForceResetAvailable(true)
        setTrackingError('Backend không còn lưu phiên này. Có thể đặt lại giao diện mà không cần hủy phiên.')
        return { lost: true, backendMissing: true }
      }
      if (result.trackingLost) {
        setTrackingLost(true)
        setBackendMissing(false)
        setTrackingError(SNAPSHOT_LOST_TRACKING_MESSAGE)
        return { lost: true, backendMissing: false }
      }
      if (result.update && result.failureCount === 0) {
        applyTrackedUpdate(result.update)
        if (wasLost && isRunning(result.update.state)) {
          setPollRecoveryToken((token) => token + 1)
        }
      }
      return noChange
    })()
    snapshotInFlightRef.current = { transferID, promise }
    void promise.then(
      () => {
        if (snapshotInFlightRef.current?.promise === promise) snapshotInFlightRef.current = null
      },
      () => {
        if (snapshotInFlightRef.current?.promise === promise) snapshotInFlightRef.current = null
      },
    )
    return promise
  }

  useEffect(() => {
    if (!activeTransferID || !activeRunning || !clientRef.current.getTransferSnapshot) {
      return
    }
    let cancelled = false
    let timer: ReturnType<typeof setInterval> | undefined

    const runSync = async (allowWhenLost = false) => {
      if (cancelled) return
      const { lost } = await probeSnapshot(activeTransferID, allowWhenLost)
      if (!cancelled && lost && timer !== undefined) {
        clearInterval(timer)
        timer = undefined
      }
    }

    const handleFocusOrVisible = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
        return
      }
      // Allow a single recovery probe on window focus/visibility even after
      // interval polling stopped at MAX_SNAPSHOT_RETRIES.
      void runSync(true)
    }

    timer = setInterval(() => {
      void runSync(false)
    }, SNAPSHOT_POLL_INTERVAL_MS)

    window.addEventListener('focus', handleFocusOrVisible)
    document.addEventListener('visibilitychange', handleFocusOrVisible)

    return () => {
      cancelled = true
      if (timer !== undefined) {
        clearInterval(timer)
      }
      window.removeEventListener('focus', handleFocusOrVisible)
      document.removeEventListener('visibilitychange', handleFocusOrVisible)
    }
  }, [activeTransferID, activeRunning, pollRecoveryToken])

  const beginTransfer = async (request: () => Promise<string>, fallbackError: string) => {
    // Buffer events by ID before Start* resolves and never let a previous
    // terminal session block a newly started session.
    trackerRef.current.beginStart()
    snapshotInFlightRef.current = null
    setPending(true)
    setTrackingLost(false)
    setBackendMissing(false)
    setTrackingError('')
    setForceResetAvailable(false)
    setAppError('')
    setConnectionCode('')
    setUpdate(undefined)
    try {
      const transferID = await request()
      const { update: started } = trackerRef.current.resolveStart(transferID)
      applyTrackedUpdate(started)
      void probeSnapshot(transferID, false)
    } catch (err) {
      trackerRef.current.failStart()
      setAppError(err instanceof Error ? err.message : fallbackError)
      throw err
    } finally {
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

  const handleCancelLostTransfer = async () => {
    if (!activeTransferID) return
    const transferID = activeTransferID
    let cancelIssue = ''
    try {
      const requested = await clientRef.current.cancelTransfer(transferID)
      if (!requested) {
        cancelIssue = 'Backend không chấp nhận yêu cầu hủy (phiên đã kết thúc, đang công bố hoặc không còn trong RAM).'
      }
    } catch (err) {
      cancelIssue = err instanceof Error ? err.message : 'Không thể gửi yêu cầu hủy phiên.'
    }
    // Recover the actual native state before ever offering a UI-only reset.
    // An accepted cancellation is asynchronous and is not a terminal update.
    const probe = await probeSnapshot(transferID, true)
    const current = trackerRef.current.current()
    const stillLost = !probe.backendMissing && current?.transferID === transferID &&
      isRunning(current.state) &&
      trackerRef.current.snapshotFailures() >= MAX_SNAPSHOT_RETRIES
    if (cancelIssue && stillLost) {
      setTrackingError(cancelIssue)
      setForceResetAvailable(true)
    } else if (cancelIssue && !probe.backendMissing &&
      current?.transferID === transferID && isRunning(current.state)) {
      // A healthy subsequent snapshot or native event clears this transient notice.
      setTrackingError(cancelIssue)
    }
  }

  const handleReset = (force = false) => {
    if (activeRunning && !(force && trackingLost && forceResetAvailable)) {
      setAppError('Phiên truyền vẫn có thể đang chạy. Hãy hủy phiên và chờ xác nhận trước khi đặt lại.')
      return
    }
    trackerRef.current.reset()
    snapshotInFlightRef.current = null
    setTrackingLost(false)
    setBackendMissing(false)
    setTrackingError('')
    setForceResetAvailable(false)
    setUpdate(undefined)
    setConnectionCode('')
    setAppError(force && !backendMissing
      ? 'Chỉ đặt lại giao diện, chưa xác nhận backend đã dừng. Phiên mới có thể bị từ chối BUSY; hãy kiểm tra lại engine trước khi thử.'
      : '')
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
          {displayedError && (
            <div className="error-alert top-alert" role="alert">
              <span className="error-icon">⚠</span>
              <span>{displayedError}</span>
              {trackingLost && activeTransferID ? (
                <div className="alert-actions">
                  <button
                    type="button"
                    className="btn-secondary btn-alert-action"
                    onClick={() => {
                      void probeSnapshot(activeTransferID, true)
                    }}
                  >
                    Thử đồng bộ lại
                  </button>
                  <button
                    type="button"
                    className="btn-secondary btn-alert-action"
                    onClick={() => void handleCancelLostTransfer()}
                  >
                    Yêu cầu hủy phiên
                  </button>
                  {forceResetAvailable && (
                    <>
                      <button
                        type="button"
                        className="btn-secondary btn-alert-action"
                        onClick={() => handleReset(true)}
                      >
                        {backendMissing ? 'Đặt lại giao diện' : 'Buộc đặt lại giao diện'}
                      </button>
                      {!backendMissing && (
                        <span>Chỉ đặt lại màn hình; backend có thể vẫn đang truyền và từ chối phiên mới (BUSY).</span>
                      )}
                    </>
                  )}
                </div>
              ) : (
                <button
                  type="button"
                  className="btn-close-alert"
                  onClick={() => { setAppError(''); setTrackingError('') }}
                  aria-label="Đóng thông báo lỗi"
                >
                  ✕
                </button>
              )}
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
              aria-pressed={useMock}
              onClick={() => {
                // Never detach the live engine listener while a transfer is active.
                if (busy) return
                setUseMock(!useMock)
                handleReset()
              }}
              disabled={busy}
              title={busy ? 'Không thể đổi engine trong khi truyền tệp' : undefined}
            >
              {useMock ? 'Chuyển sang Engine thật' : 'Chuyển sang Chế độ mô phỏng'}
            </button>
          )}
        </div>
      </footer>
    </main>
  )
}
