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
    // On desktop, never copy secrets through WebView clipboard APIs (CC-TM-001).
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
    <div className={`transfer-card card-state-${state.toLowerCase()}`} aria-live="polite">
      {/* State Header */}
      <div className="card-header">
        <div className="state-badge">
          <span className={`status-dot dot-${state.toLowerCase()} ${active ? 'pulse' : ''}`} />
          <span className="state-name">{STATE_LABELS[state] || state}</span>
        </div>
        {active && (
          <button
            type="button"
            className="btn-cancel"
            onClick={onCancel}
            title="Hủy phiên truyền tệp này"
          >
            Hủy phiên
          </button>
        )}
      </div>

      {/* Description / Message */}
      <p className="state-description">
        {message || (error ? error.message : STATE_DESCRIPTIONS[state])}
      </p>

      {/* Code Display for Sender in WAITING_FOR_PEER */}
      {direction === 'send' && codeToShow && (state === 'WAITING_FOR_PEER' || state === 'PREPARING') && (
        <div className="code-box">
          <div className="code-label">MÃ KẾT NỐI:</div>
          <div className="code-row">
            <code className="code-value">{codeToShow}</code>
            <button
              type="button"
              className={`btn-copy ${copied ? 'copied' : ''}`}
              onClick={() => void handleCopyCode()}
              title="Sao chép mã an toàn (loại trừ lịch sử clipboard)"
            >
              {copied ? '✓ Đã sao chép' : '📋 Sao chép mã'}
            </button>
          </div>
          {copyError && (
            <div className="error-alert" role="alert" style={{ marginTop: '0.5rem' }}>
              <span className="error-icon">⚠</span>
              <span>{copyError}</span>
            </div>
          )}
          <p className="code-hint">
            Chia sẻ mã này qua kênh tin cậy cho người nhận. Quá trình truyền sẽ bắt đầu ngay khi người nhận kết nối.
          </p>
        </div>
      )}

      {/* Progress Bar */}
      {(state === 'TRANSFERRING' || state === 'FINALIZING') && (
        <div className="progress-section">
          <div className="progress-bar-container">
            {computedProgress !== undefined ? (
              <div
                className="progress-bar-fill"
                style={{ width: `${Math.min(100, Math.max(0, computedProgress))}%` }}
                role="progressbar"
                aria-label="Tiến độ truyền tệp"
                aria-valuenow={Math.round(computedProgress)}
                aria-valuemin={0}
                aria-valuemax={100}
              />
            ) : (
              <div className="progress-bar-indeterminate" />
            )}
          </div>
          {computedProgress !== undefined && (
            <div className="progress-percent">{Math.round(computedProgress)}%</div>
          )}
        </div>
      )}

      {/* Transfer Metrics (during transfer or final) */}
      {(state === 'TRANSFERRING' || state === 'FINALIZING' || state === 'COMPLETED') && (
        <div className="metrics-grid">
          {currentFile && (
            <div className="metric-item metric-file">
              <span className="metric-label">Đang truyền:</span>
              <span className="metric-value file-truncate" title={currentFile}>
                {currentFile}
              </span>
            </div>
          )}
          {bytesTransferred !== undefined && (
            <div className="metric-item">
              <span className="metric-label">Dung lượng:</span>
              <span className="metric-value">
                {formatBytes(bytesTransferred)}
                {bytesTotal !== undefined && ` / ${formatBytes(bytesTotal)}`}
              </span>
            </div>
          )}
          {bytesPerSecond !== undefined && bytesPerSecond > 0 && (
            <div className="metric-item">
              <span className="metric-label">Tốc độ:</span>
              <span className="metric-value">{formatSpeed(bytesPerSecond)}</span>
            </div>
          )}
          {eta && (
            <div className="metric-item">
              <span className="metric-label">Thời gian còn lại:</span>
              <span className="metric-value">{eta}</span>
            </div>
          )}
        </div>
      )}

      {/* Safety warnings for partial output or pending cleanup */}
      {partialOutput && (
        <div className="warning-alert" role="alert">
          <span className="error-icon">⚠</span>
          <span>Thư mục đích có thể chứa tệp chưa hoàn chỉnh sau lỗi công bố. Hãy kiểm tra thủ công trước khi thử lại.</span>
        </div>
      )}
      {cleanupPending && (
        <div className="warning-alert" role="alert">
          <span className="error-icon">⚠</span>
          <span>Hệ thống chưa xác nhận dọn xong tài nguyên phiên trước. Phiên mới đang tạm khóa an toàn cho tới khi dọn hoàn tất hoặc khởi động lại ứng dụng.</span>
        </div>
      )}

      {/* Completion or Error States */}
      {state === 'COMPLETED' && (
        <div className="result-banner result-success">
          <div className="result-icon">✓</div>
          <div className="result-content">
            <h4>Truyền tệp thành công!</h4>
            <p>Tất cả dữ liệu đã được truyền mã hóa và kiểm tra toàn vẹn theo giao thức Croc.</p>
          </div>
          <button type="button" className="btn-primary" onClick={onReset}>
            {direction === 'send' ? 'Gửi tệp khác' : 'Nhận tệp khác'}
          </button>
        </div>
      )}

      {state === 'FAILED' && (
        <div className="result-banner result-failed">
          <div className="result-icon">✕</div>
          <div className="result-content">
            <h4>Truyền tệp thất bại</h4>
            <p>{error?.message || message || 'Quá trình kết nối hoặc truyền dữ liệu gặp sự cố.'}</p>
          </div>
          <button type="button" className="btn-secondary" onClick={onReset}>
            Thử lại
          </button>
        </div>
      )}

      {state === 'CANCELLED' && (
        <div className="result-banner result-cancelled">
          <div className="result-icon">⊘</div>
          <div className="result-content">
            <h4>Phiên đã được hủy</h4>
            <p>Tiến trình đã được dừng và giải phóng tài nguyên hệ thống an toàn.</p>
          </div>
          <button type="button" className="btn-secondary" onClick={onReset}>
            Đặt lại
          </button>
        </div>
      )}
    </div>
  )
}
