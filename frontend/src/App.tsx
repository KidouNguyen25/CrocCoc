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
import { SidebarRail } from './components/ModeSwitcher.tsx'
import type { AppRoute } from './components/ModeSwitcher.tsx'
import { SendPanel } from './components/SendPanel.tsx'
import { ReceivePanel } from './components/ReceivePanel.tsx'
import { STATE_LABELS } from './utils/formatters.ts'

type SnapshotProbeResult = { lost: boolean; backendMissing: boolean }

export interface SessionHistoryItem {
  id: string
  direction: 'send' | 'receive'
  label: string
  code: string
  timestamp: string
  status: string
  statusText: string
}

export function App() {
  const [route, setRoute] = useState<AppRoute>('gui')
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
  const [sessions, setSessions] = useState<SessionHistoryItem[]>([])

  // Persistent mock client instance for browser preview
  const mockClient = useMemo(() => createMockTransferClient(), [])
  const clientRef = useRef<TransferClient>(mockClient)
  const trackerRef = useRef(createTransferSessionTracker())
  const snapshotInFlightRef = useRef<{ transferID: string; promise: Promise<SnapshotProbeResult> } | null>(null)

  // Listen to hash change for navigation
  useEffect(() => {
    const handleHash = () => {
      const hash = window.location.hash.replace(/^#/, '')
      if (hash === 'gui' || hash === 'nhan' || hash === 'phien' || hash === 'engine') {
        setRoute(hash as AppRoute)
      }
    }
    handleHash()
    window.addEventListener('hashchange', handleHash)
    return () => window.removeEventListener('hashchange', handleHash)
  }, [])

  const changeRoute = (nextRoute: AppRoute) => {
    setRoute(nextRoute)
    window.location.hash = nextRoute
  }

  const recordSession = (
    transferID: string,
    direction: 'send' | 'receive',
    label: string,
    code: string,
    state: string,
  ) => {
    const timeStr = new Date().toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    setSessions((prev) => {
      const existingIdx = prev.findIndex((s) => s.id === transferID)
      const statusText = STATE_LABELS[state as keyof typeof STATE_LABELS] || state
      if (existingIdx >= 0) {
        const copy = [...prev]
        copy[existingIdx] = {
          ...copy[existingIdx],
          code: code || copy[existingIdx].code,
          status: state,
          statusText,
        }
        return copy
      }
      return [
        {
          id: transferID,
          direction,
          label,
          code: code || '—',
          timestamp: timeStr,
          status: state,
          statusText,
        },
        ...prev,
      ]
    })
  }

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

    if (next.transferID) {
      recordSession(
        next.transferID,
        route === 'nhan' ? 'receive' : 'send',
        route === 'nhan' ? 'Nhận tệp' : 'Gửi tệp',
        next.connectionCode || connectionCode,
        next.state,
      )
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
            setHostTitle(info.message ? `Ứng dụng desktop · ${info.message}` : 'Ứng dụng desktop · Chưa phát hiện Croc')
          } else if (info.relayLabel) {
            setHostTitle(`Ứng dụng desktop (Wails IPC) · ${info.relayLabel}`)
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
        setTrackingError(
          'Backend không còn lưu phiên này; chưa xác định truyền thành công hay thất bại. Hãy kiểm tra thư mục gửi/nhận trước khi đặt lại giao diện.',
        )
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

  const beginTransfer = async (
    request: () => Promise<string>,
    fallbackError: string,
    direction: 'send' | 'receive',
    label: string,
  ) => {
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
      recordSession(transferID, direction, label, '', started?.state || 'PREPARING')
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
    beginTransfer(
      () => clientRef.current.startSend(paths),
      'Lỗi khi bắt đầu phiên gửi.',
      'send',
      `Gửi ${paths.length} mục`,
    )

  const handleStartReceive = (code: string, outputDir: string) =>
    beginTransfer(
      () => clientRef.current.startReceive(code, outputDir),
      'Lỗi khi bắt đầu phiên nhận.',
      'receive',
      'Nhận tệp',
    )

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
    const probe = await probeSnapshot(transferID, true)
    const current = trackerRef.current.current()
    const stillLost =
      !probe.backendMissing &&
      current?.transferID === transferID &&
      isRunning(current.state) &&
      trackerRef.current.snapshotFailures() >= MAX_SNAPSHOT_RETRIES
    if (cancelIssue && stillLost) {
      setTrackingError(cancelIssue)
      setForceResetAvailable(true)
    } else if (
      cancelIssue &&
      !probe.backendMissing &&
      current?.transferID === transferID &&
      isRunning(current.state)
    ) {
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
    setAppError(
      force && !backendMissing
        ? 'Chỉ đặt lại giao diện, chưa xác nhận backend đã dừng. Phiên mới có thể bị từ chối BUSY; hãy kiểm tra lại engine trước khi thử.'
        : '',
    )
  }

  return (
    <div className="shell">
      {/* SVG Symbols Sprite */}
      <svg className="symbols" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width="0" height="0">
        <symbol id="i-send" viewBox="0 0 24 24">
          <path d="M3 11.5 21 3l-5 18-4-7-9-2.5ZM12 14l4-5" />
        </symbol>
        <symbol id="i-receive" viewBox="0 0 24 24">
          <path d="M3 13h5l2 3h4l2-3h5M5 6h14l2 7v6H3v-6l2-7Z" />
        </symbol>
        <symbol id="i-history" viewBox="0 0 24 24">
          <path d="M3 12a9 9 0 1 1 2.7 6.4M3 5v7h7m2-5v5l3 2" />
        </symbol>
        <symbol id="i-engine" viewBox="0 0 24 24">
          <path d="M4 4h16v16H4zM8 9l3 3-3 3m5 0h3" />
        </symbol>
        <symbol id="i-file" viewBox="0 0 24 24">
          <path d="M6 3h8l4 4v14H6zM14 3v5h4M9 12h6m-6 4h6" />
        </symbol>
        <symbol id="i-arrow" viewBox="0 0 24 24">
          <path d="M4 12h15m-6-6 6 6-6 6" />
        </symbol>
        <symbol id="i-copy" viewBox="0 0 24 24">
          <rect x="8" y="8" width="12" height="12" rx="2" />
          <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" />
        </symbol>
        <symbol id="i-upload" viewBox="0 0 24 24">
          <path d="M12 15V3m-5 5 5-5 5 5M4 15v5h16v-5" />
        </symbol>
        <symbol id="i-close" viewBox="0 0 24 24">
          <path d="m6 6 12 12M18 6 6 18" />
        </symbol>
        <symbol id="i-preview" viewBox="0 0 24 24">
          <path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6-10-6-10-6Z" />
          <circle cx="12" cy="12" r="2.5" />
        </symbol>
      </svg>

      {/* Topbar Header */}
      <Header
        hostTitle={hostTitle}
        crocInfo={crocInfo}
        isDesktop={isDesktop}
        onBrandClick={() => changeRoute('gui')}
      />

      <div className="layout">
        {/* Sidebar Rail Navigation */}
        <SidebarRail
          activeRoute={route}
          onRouteChange={changeRoute}
          disabled={busy}
          isDesktop={isDesktop}
          isMock={!isDesktop || useMock}
          crocAvailable={crocInfo?.available}
          relayLabel={crocInfo?.relayLabel}
        />

        {/* Main Content Screens */}
        <main>
          {displayedError && (
            <div className="error-alert top-alert" role="alert">
              <span>{displayedError}</span>
              {trackingLost && activeTransferID ? (
                <div className="alert-actions">
                  <button
                    type="button"
                    className="button secondary"
                    onClick={() => {
                      void probeSnapshot(activeTransferID, true)
                    }}
                  >
                    Thử đồng bộ lại
                  </button>
                  {!backendMissing && (
                    <button
                      type="button"
                      className="button secondary"
                      onClick={() => void handleCancelLostTransfer()}
                    >
                      Yêu cầu hủy phiên
                    </button>
                  )}
                  {forceResetAvailable && (
                    <button
                      type="button"
                      className="button secondary"
                      onClick={() => handleReset(true)}
                    >
                      {backendMissing ? 'Đặt lại giao diện' : 'Buộc đặt lại giao diện'}
                    </button>
                  )}
                </div>
              ) : (
                <button
                  type="button"
                  className="btn-close-alert"
                  onClick={() => {
                    setAppError('')
                    setTrackingError('')
                  }}
                  aria-label="Đóng thông báo lỗi"
                >
                  ✕
                </button>
              )}
            </div>
          )}

          {/* Screen 1: Gửi tệp (#gui) */}
          {route === 'gui' && (
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
          )}

          {/* Screen 2: Nhận tệp (#nhan) */}
          {route === 'nhan' && (
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

          {/* Screen 3: Phiên truyền (#phien) */}
          {route === 'phien' && (
            <section className="screen" id="phien" aria-labelledby="history-title">
              <header className="intro">
                <h1 id="history-title" tabIndex={-1}>
                  Phiên truyền
                </h1>
                <p>Xem các phiên truyền được tạo trong lần mở này.</p>
              </header>

              <section className="panel narrow" aria-labelledby="history-list-title">
                <div className="panel-head">
                  <h2 id="history-list-title">Lịch sử phiên truyền</h2>
                  <span className="state" id="history-count">
                    {sessions.length} phiên
                  </span>
                </div>

                {sessions.length === 0 ? (
                  <p className="empty" id="history-empty">
                    Chưa có phiên nào.{' '}
                    <button type="button" className="link-btn" onClick={() => changeRoute('gui')}>
                      Tạo mã gửi đầu tiên
                    </button>
                    .
                  </p>
                ) : (
                  <ul className="file-list history-list" id="history-list">
                    {sessions.map((item) => (
                      <li key={item.id}>
                        <div className="file-label">
                          <svg className="icon" aria-hidden="true">
                            <use href={item.direction === 'send' ? '#i-send' : '#i-receive'} />
                          </svg>
                          <div>
                            <strong>{item.label}</strong>
                            <div className="session-meta">{item.timestamp}</div>
                          </div>
                        </div>
                        <div className="file-meta">
                          {item.code && item.code !== '—' && <code>{item.code} </code>}
                          <span className={`session-badge badge-${item.status.toLowerCase()}`}>
                            {item.statusText}
                          </span>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </section>
          )}

          {/* Screen 4: Croc engine (#engine) */}
          {route === 'engine' && (
            <section className="screen" id="engine" aria-labelledby="engine-title">
              <header className="intro">
                <h1 id="engine-title" tabIndex={-1}>
                  Croc engine
                </h1>
                <p>Thông tin kết nối và cấu hình động cơ truyền tệp.</p>
              </header>

              <section className="panel narrow" aria-labelledby="engine-status">
                <div className="panel-head">
                  <h2 id="engine-status">Trạng thái</h2>
                  <span className="state" data-ready={crocInfo?.available ? '' : undefined}>
                    {crocInfo?.available ? 'Sẵn sàng' : 'Chưa kết nối'}
                  </span>
                </div>

                <dl className="spec-list">
                  <div>
                    <dt>Chương trình Croc</dt>
                    <dd>
                      {crocInfo?.version
                        ? `Croc ${crocInfo.version}`
                        : isDesktop
                          ? 'Chưa phát hiện'
                          : 'Chế độ mô phỏng (Mock)'}
                    </dd>
                  </div>
                  <div>
                    <dt>Đường dẫn cài đặt</dt>
                    <dd>{isDesktop ? 'Tích hợp sẵn (Worker Go)' : 'Trình duyệt Web (Mock)'}</dd>
                  </div>
                  <div>
                    <dt>Máy chủ Relay</dt>
                    <dd>{crocInfo?.relayLabel || 'Relay công cộng (Mặc định)'}</dd>
                  </div>
                  <div>
                    <dt>Môi trường chạy</dt>
                    <dd>{isDesktop ? 'Ứng dụng desktop (Wails IPC)' : 'Bản xem trước trình duyệt'}</dd>
                  </div>
                  {crocInfo?.message && (
                    <div>
                      <dt>Thông tin engine</dt>
                      <dd>{crocInfo.message}</dd>
                    </div>
                  )}
                </dl>

                {hasDesktopRuntime && (
                  <div
                    style={{
                      marginTop: '20px',
                      paddingTop: '16px',
                      borderTop: '1px solid var(--rule)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      flexWrap: 'wrap',
                      gap: '12px',
                    }}
                  >
                    <span style={{ fontSize: '13px', color: 'var(--muted)' }}>
                      Chuyển đổi engine cho mục đích kiểm thử / gỡ lỗi:
                    </span>
                    <button
                      type="button"
                      className="button secondary"
                      aria-pressed={useMock}
                      onClick={() => {
                        if (busy) return
                        setUseMock(!useMock)
                        handleReset()
                      }}
                      disabled={busy}
                      title={busy ? 'Không thể đổi engine trong khi truyền tệp' : undefined}
                    >
                      {useMock ? 'Chuyển sang Engine thật (Wails)' : 'Chuyển sang Chế độ mô phỏng'}
                    </button>
                  </div>
                )}
              </section>
            </section>
          )}

          {/* Footer note matching mockup */}
          <footer className="main-foot">
            <svg className="icon" aria-hidden="true">
              <use href="#i-preview" />
            </svg>
            <span>
              {isDesktop && !useMock
                ? 'CrocCoc Desktop v0.1.0 • Chuẩn truyền Croc PAKE'
                : 'Bản mô phỏng không truyền dữ liệu.'}
            </span>
          </footer>
        </main>
      </div>
    </div>
  )
}
