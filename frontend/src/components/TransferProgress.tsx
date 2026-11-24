import { useState } from 'react'
import type { TransferUpdate } from '../transfer.ts'
import { isRunning } from '../transfer.ts'
import {
  computeProgressPercent,
  formatBytes,
  formatSpeed,
  calculateETA,
  STATE_LABELS,
  STATE_DESCRIPTIONS,
} from '../utils/formatters.ts'

interface TransferProgressProps {
  update?: TransferUpdate
  connectionCode?: string
  direction: 'send' | 'receive'
  isDesktop?: boolean
  onCopySendCode?: () => Promise<void>
  onCancel: () => void
  onReset: () => void
}

export function TransferProgress({
  update,
  connectionCode,
  direction,
  isDesktop = false,
  onCopySendCode,
  onCancel,
  onReset,
}: TransferProgressProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')

  if (!update || update.state === 'IDLE') return null

  const {
    state,
    progress,
    bytesTransferred,
    bytesTotal,
    bytesPerSecond,
    currentFile,
    message,
    error,
    partialOutput,
    cleanupPending,
  } = update

  const computedProgress = computeProgressPercent(bytesTransferred, bytesTotal) ?? progress
  const active = isRunning(state)
  const codeToShow = update.connectionCode || connectionCode
  const eta = calculateETA(bytesTransferred, bytesTotal, bytesPerSecond)

  const handleCopyCode = async () => {
    if (!codeToShow) return
    setCopyError('')
    if (isDesktop) {
      if (!onCopySendCode) {
        setCopyError('Chưa xác minh được chế độ sao chép an toàn. Vui lòng đọc mã trực tiếp trên màn hình.')
        return
      }
      try {
        await onCopySendCode()
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      } catch (err) {
        setCopyError(
          err instanceof Error
            ? err.message
            : 'Không thể sao chép mã an toàn (COPY_UNSAFE). Vui lòng đọc mã trực tiếp trên màn hình.',
        )
      }
      return
    }
    try {
      await navigator.clipboard.writeText(codeToShow)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopyError('Không thể sao chép trong chế độ xem trước. Vui lòng chép thủ công.')
    }
  }

  return (
    <div className={`transfer-status-box card-state-${state.toLowerCase()}`} aria-live="polite">
      {/* State Header */}
      <div className="status-header">
        <div className="status-badge-inline">
          <span className={`status-dot dot-${state.toLowerCase()} ${active ? 'pulse' : ''}`} />
          <span className="state-name">{STATE_LABELS[state] || state}</span>
        </div>
        {active ? (
          <button
            type="button"
            className="button danger"
            onClick={onCancel}
            title="Hủy phiên truyền tệp này"
          >
            Hủy phiên
          </button>
        ) : (
          <button
            type="button"
            className="button secondary"
            onClick={onReset}
            title="Tạo phiên mới"
          >
            {direction === 'send' ? 'Gửi tệp khác' : 'Nhận tệp khác'}
          </button>
        )}
      </div>

      {/* Description / Message */}
      <p className="status-desc">
        {message || (error ? error.message : STATE_DESCRIPTIONS[state])}
      </p>

      {/* Progress Bar during active transfer */}
      {(state === 'TRANSFERRING' || state === 'FINALIZING') && (
        <div className="progress-track" role="progressbar" aria-valuenow={computedProgress ? Math.round(computedProgress) : undefined} aria-valuemin={0} aria-valuemax={100}>
          {computedProgress !== undefined ? (
            <div
              className="progress-bar-fill"
              style={{ width: `${Math.min(100, Math.max(0, computedProgress))}%` }}
            />
          ) : (
            <div className="progress-bar-indeterminate" />
          )}
        </div>
      )}

      {/* Transfer Metrics */}
      {(state === 'TRANSFERRING' || state === 'FINALIZING' || state === 'COMPLETED') && (
        <div className="transfer-metrics">
          {currentFile && (
            <span title={currentFile}>Tệp: {currentFile}</span>
          )}
          {bytesTransferred !== undefined && (
            <span>
              {formatBytes(bytesTransferred)}
              {bytesTotal !== undefined && ` / ${formatBytes(bytesTotal)}`}
              {computedProgress !== undefined && ` (${Math.round(computedProgress)}%)`}
            </span>
          )}
          {bytesPerSecond !== undefined && bytesPerSecond > 0 && (
            <span>Tốc độ: {formatSpeed(bytesPerSecond)}</span>
          )}
          {eta && <span>Còn ~{eta}</span>}
        </div>
      )}

      {/* Copy Code banner for Sender if waiting */}
      {direction === 'send' && codeToShow && (state === 'WAITING_FOR_PEER' || state === 'PREPARING') && (
        <div style={{ marginTop: '8px' }}>
          {copyError && (
            <div className="error-alert" role="alert" style={{ marginTop: '8px', marginBottom: '8px' }}>
              <span>{copyError}</span>
            </div>
          )}
          <p className="panel-note" style={{ margin: '4px 0 0' }}>
            Chia sẻ mã với người nhận. Quá trình truyền sẽ tự động bắt đầu khi người nhận kết nối.
          </p>
        </div>
      )}

      {/* Warnings */}
      {partialOutput && (
        <div className="error-alert" role="alert" style={{ marginTop: '8px', marginBottom: 0 }}>
          <span>⚠ Thư mục đích có thể chứa tệp chưa hoàn chỉnh. Hãy kiểm tra trước khi thử lại.</span>
        </div>
      )}
      {cleanupPending && (
        <div className="error-alert" role="alert" style={{ marginTop: '8px', marginBottom: 0 }}>
          <span>⚠ Hệ thống đang dọn tài nguyên phiên trước. Phiên mới tạm thời bị khóa.</span>
        </div>
      )}
    </div>
  )
}
