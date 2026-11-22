import { useEffect, useRef } from 'react'
import type { ReceiveOffer } from '../transfer.ts'
import { formatBytes } from '../utils/formatters.ts'

interface ReceiveOfferDialogProps {
  offer: ReceiveOffer
  onDecide: (accept: boolean) => void
  disabled?: boolean
}

export function ReceiveOfferDialog({ offer, onDecide, disabled = false }: ReceiveOfferDialogProps) {
  const { fileCount, totalBytes, items, expiresAt } = offer
  const dialogRef = useRef<HTMLDivElement>(null)
  const previouslyFocused = useRef<HTMLElement | null>(null)

  useEffect(() => {
    // Save previously focused element
    previouslyFocused.current = document.activeElement as HTMLElement | null

    // Focus trap
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !disabled) {
        e.preventDefault()
        onDecide(false)
        return
      }
      if (e.key === 'Tab') {
        const focusableElements = dialogRef.current?.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        const firstElement = focusableElements?.[0] as HTMLElement | undefined
        const lastElement = focusableElements?.[focusableElements.length - 1] as HTMLElement | undefined

        if (e.shiftKey && document.activeElement === firstElement) {
          e.preventDefault()
          lastElement?.focus()
        } else if (!e.shiftKey && (document.activeElement as HTMLElement | null) === lastElement) {
          e.preventDefault()
          firstElement?.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    // Focus first focusable element (Accept button)
    const focusTarget = dialogRef.current?.querySelector<HTMLElement>('button[autoFocus]') ||
      dialogRef.current?.querySelector<HTMLElement>('.btn-accept')
    setTimeout(() => focusTarget?.focus(), 0)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      // Restore focus to previously focused element
      previouslyFocused.current?.focus()
    }
  }, [disabled])

  const expiryLabel = expiresAt
    ? new Date(expiresAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : ''

  return (
    <div
      ref={dialogRef}
      className="offer-dialog-card"
      role="dialog"
      aria-modal="true"
      aria-labelledby="offer-title"
    >
      <div className="offer-header">
        <span className="offer-badge">YÊU CẦU TRUYỀN TỆP</span>
        <h3 id="offer-title">Bên gửi muốn gửi cho bạn:</h3>
        <p className="offer-summary">
          <strong>{fileCount}</strong> tệp / thư mục • Tổng dung lượng: <strong>{formatBytes(totalBytes)}</strong>
          {expiryLabel && <> • Hết hạn lúc: <strong>{expiryLabel}</strong></>}
        </p>
      </div>

      <div className="offer-items-box">
        <ul className="offer-items-list" role="list">
          {items.map((item, index) => (
            <li key={index} className="offer-item">
              <span className="offer-item-icon">{item.type === 'directory' ? '📁' : '📄'}</span>
              <span className="offer-item-path" title={item.path}>{item.path}</span>
              {item.sizeBytes !== undefined && item.sizeBytes !== '' && (
                <span className="offer-item-size">{formatBytes(item.sizeBytes)}</span>
              )}
            </li>
          ))}
        </ul>
      </div>

      <p className="offer-notice">
        Vui lòng xác nhận bạn tin tưởng nguồn gửi này trước khi tải dữ liệu về máy. Nhấn Esc để từ chối.
      </p>

      <div className="offer-actions">
        <button
          type="button"
          className="btn-primary btn-accept"
          onClick={() => onDecide(true)}
          disabled={disabled}
          autoFocus
        >
          ✓ Đồng ý nhận tệp
        </button>
        <button
          type="button"
          className="btn-cancel btn-decline"
          onClick={() => onDecide(false)}
          disabled={disabled}
        >
          ✕ Từ chối
        </button>
      </div>
    </div>
  )
}